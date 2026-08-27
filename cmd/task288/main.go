// 命令 task288 启动微流控液滴分裂谱系复核服务。
//
// 用法：
//
//	go run ./cmd/task288 --addr :8080 --db dropletlineage.db
//	go run ./cmd/task288 --smoke-test            # 不启动长驻服务，自检后退出
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"task288-dropletlineage/internal/httpapi"
	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/service"
	"task288-dropletlineage/internal/store"
	"task288-dropletlineage/internal/verdict"
)

func main() {
	var (
		addr      = flag.String("addr", ":8080", "listen address")
		dbPath    = flag.String("db", "dropletlineage.db", "sqlite database path")
		smokeTest = flag.Bool("smoke-test", false, "run smoke test and exit")
	)
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	app := service.New(db)
	if *smokeTest {
		if err := runSmokeTest(context.Background(), app, *dbPath); err != nil {
			log.Fatalf("smoke test failed: %v", err)
		}
		fmt.Println("smoke-test OK")
		return
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.New(app).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("task288-dropletlineage listening on %s (db=%s)", *addr, *dbPath)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

// runSmokeTest 执行端到端自检，覆盖：
//  1. 创建批次、导入帧与观测（含帧序/坐标校验、幂等）；
//  2. 跨帧跟踪：正常关联、速度外推、遮挡断裂候选；
//  3. 分裂/合并守恒校验（体积 + 荧光标记）；
//  4. 裁决确认与版本冻结发布；
//  5. 关闭并重新打开同一数据库，验证持久化与重启恢复。
func runSmokeTest(ctx context.Context, app *service.App, dbPath string) error {
	smokeDB := dbPath + ".smoke"
	_ = os.Remove(smokeDB)

	db, err := store.Open(smokeDB)
	if err != nil {
		return fmt.Errorf("open smoke db: %w", err)
	}
	smokeApp := service.New(db)

	// 1. 创建批次（1000×500 通道）
	batch, err := smokeApp.CreateBatch(ctx, "smoke-droplet-lineage", 1000, 500)
	if err != nil {
		return fmt.Errorf("create batch: %w", err)
	}
	if batch.Status != model.BatchImporting {
		return fmt.Errorf("batch status = %s, want importing", batch.Status)
	}

	// 2. 导入帧 1..4（帧序单调）
	for _, f := range []struct {
		seq     int
		tMs     int64
		channel string
	}{
		{1, 0, "ch1"}, {2, 33, "ch1"}, {3, 66, "ch1"}, {4, 99, "ch1"},
	} {
		if _, created, err := smokeApp.Ingest.ImportFrame(ctx, batch.ID, f.seq, f.tMs, f.channel, "img-"+fmt.Sprint(f.seq)); err != nil {
			return fmt.Errorf("import frame %d: %w", f.seq, err)
		} else if !created {
			return fmt.Errorf("frame %d not created", f.seq)
		}
	}
	// 帧序倒退在跳号场景被拒绝（此处 1..4 均已存在，重复导入走幂等分支，
	// 跳号补旧帧的 ErrSequenceRegression 由 ingest 单测覆盖）
	if _, created, err := smokeApp.Ingest.ImportFrame(ctx, batch.ID, 4, 99, "ch1", "img-4"); err != nil || created {
		return fmt.Errorf("frame idempotency: created=%v err=%v", created, err)
	}

	// 3. 导入观测
	type obsIn struct {
		frame int
		key   string
		x, y  float64
		r     float64
		mark  string
		I     float64
	}
	// 场景：A/B 正常跟踪；P 帧1 单滴 → 帧2 分裂 C1/C2；
	// M1/M2 帧1 → 帧2 合并 MC；A 帧3 缺失、帧4 远处重现（遮挡断裂）。
	obs := []obsIn{
		{1, "A", 100, 100, 3, "GFP", 100},
		{1, "B", 200, 100, 3, "GFP", 80},
		{1, "P", 500, 300, 6, "RFP", 100},
		{1, "M1", 700, 100, 4, "GFP", 60},
		{1, "M2", 700, 150, 4.5, "GFP", 40},
		{2, "A", 120, 100, 3, "GFP", 100},
		{2, "B", 210, 100, 3, "GFP", 80},
		{2, "C1", 490, 280, 4.762, "RFP", 55},
		{2, "C2", 510, 300, 4.762, "RFP", 40},
		{2, "MC", 700, 120, 5.37, "GFP", 95},
		{3, "B", 220, 100, 3, "GFP", 80},
		{3, "C1", 500, 280, 4.762, "RFP", 55},
		{3, "C2", 520, 300, 4.762, "RFP", 40},
		{3, "MC", 700, 140, 5.37, "GFP", 95},
		{4, "B", 230, 100, 3, "GFP", 80},
		{4, "C1", 510, 280, 4.762, "RFP", 55},
		{4, "C2", 530, 300, 4.762, "RFP", 40},
		{4, "MC", 700, 160, 5.37, "GFP", 95},
		{4, "A", 400, 300, 3, "GFP", 100}, // A 在帧4 远处重现 → 断裂候选
	}
	obsByFrame := map[string]string{}
	for _, o := range obs {
		obsID, created, err := smokeApp.Ingest.ImportObservation(ctx, batch.ID, o.frame, o.key, o.x, o.y, o.r, o.I, o.mark)
		if err != nil {
			return fmt.Errorf("import obs %s@f%d: %w", o.key, o.frame, err)
		}
		if !created {
			return fmt.Errorf("obs %s@f%d not created", o.key, o.frame)
		}
		obsByFrame[o.key+"@"+fmt.Sprint(o.frame)] = obsID.ID
	}
	// 同帧同液滴幂等
	if _, created, err := smokeApp.Ingest.ImportObservation(ctx, batch.ID, 1, "A", 100, 100, 3, 100, "GFP"); err != nil || created {
		return fmt.Errorf("obs idempotency: created=%v err=%v", created, err)
	}
	// 坐标越界被拒绝
	if _, _, err := smokeApp.Ingest.ImportObservation(ctx, batch.ID, 1, "X", 2000, 100, 3, 10, "GFP"); err != model.ErrCoordOutOfRange {
		return fmt.Errorf("coord out of range: err = %v", err)
	}

	// 4. 跟踪：期望 A 断裂（遮挡）、B/C1/C2/MC 完整、P/M1/M2 单帧
	tr, err := smokeApp.RunTracking(ctx, batch.ID)
	if err != nil {
		return fmt.Errorf("run tracking: %w", err)
	}
	if tr.BrokenCount != 1 {
		return fmt.Errorf("broken tracks = %d, want 1", tr.BrokenCount)
	}
	if len(tr.Hints) != 1 || tr.Hints[0].DropletKey != "A" {
		return fmt.Errorf("occlusion hints = %+v, want single hint for A", tr.Hints)
	}
	if _, err := smokeApp.TransitionBatch(ctx, batch.ID, model.BatchTracking); err != nil {
		return fmt.Errorf("transition to tracking: %w", err)
	}

	// 5. 自动申报的遮挡事件存在
	events, err := smokeApp.Events.ListByBatch(ctx, batch.ID)
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}
	if len(events) != 1 || events[0].Kind != "occlusion" {
		return fmt.Errorf("auto occlusion events = %d (kind=%s), want 1 occlusion", len(events), eventKindOf(events))
	}

	// 6. 申报分裂事件 P → C1 + C2，守恒应通过
	splitEv, err := smokeApp.Verdict.DeclareSplit(ctx, batch.ID,
		[]*model.Observation{mustGetObs(smokeApp, ctx, obsByFrame["P@1"])},
		[]*model.Observation{
			mustGetObs(smokeApp, ctx, obsByFrame["C1@2"]),
			mustGetObs(smokeApp, ctx, obsByFrame["C2@2"]),
		}, 0.05, "P splits into C1 and C2")
	if err != nil {
		return fmt.Errorf("declare split: %w", err)
	}
	sr, err := smokeApp.Conservation.CheckEvent(ctx, splitEv.ID, 0.80)
	if err != nil {
		return fmt.Errorf("check split: %w", err)
	}
	if sr.Status != model.EventSplit || !sr.Passed {
		return fmt.Errorf("split conservation: status=%s passed=%v, want split/true", sr.Status, sr.Passed)
	}

	// 7. 申报合并事件 M1 + M2 → MC，守恒应通过
	mergeEv, err := smokeApp.Verdict.DeclareMerge(ctx, batch.ID,
		[]*model.Observation{
			mustGetObs(smokeApp, ctx, obsByFrame["M1@1"]),
			mustGetObs(smokeApp, ctx, obsByFrame["M2@1"]),
		},
		[]*model.Observation{mustGetObs(smokeApp, ctx, obsByFrame["MC@2"])}, 0.05, "M1 and M2 merge into MC")
	if err != nil {
		return fmt.Errorf("declare merge: %w", err)
	}
	mr, err := smokeApp.Conservation.CheckEvent(ctx, mergeEv.ID, 0.80)
	if err != nil {
		return fmt.Errorf("check merge: %w", err)
	}
	if mr.Status != model.EventMerge || !mr.Passed {
		return fmt.Errorf("merge conservation: status=%s passed=%v, want merge/true", mr.Status, mr.Passed)
	}

	// 8. 裁决：确认分裂、合并与遮挡
	for _, evID := range []string{splitEv.ID, mergeEv.ID, events[0].ID} {
		if _, err := smokeApp.Verdict.Decide(ctx, verdictDecide(evID, true)); err != nil {
			return fmt.Errorf("confirm event %s: %w", evID, err)
		}
	}
	// 重复裁决被拒绝
	if _, err := smokeApp.Verdict.Decide(ctx, verdictDecide(splitEv.ID, true)); err != model.ErrEventClosed {
		return fmt.Errorf("double decide: err = %v, want ErrEventClosed", err)
	}
	if _, err := smokeApp.TransitionBatch(ctx, batch.ID, model.BatchReview); err != nil {
		return fmt.Errorf("transition to review: %w", err)
	}

	// 9. 版本：草稿 → 冻结 → 批次发布
	ver, err := smokeApp.Version.CreateDraft(ctx, batch.ID, "v1 smoke")
	if err != nil {
		return fmt.Errorf("create version: %w", err)
	}
	if ver.EventCount != 3 {
		return fmt.Errorf("version event count = %d, want 3", ver.EventCount)
	}
	frozen, err := smokeApp.Version.Publish(ctx, ver.ID, model.VersionFrozen)
	if err != nil {
		return fmt.Errorf("publish version: %w", err)
	}
	if frozen.Status != model.VersionFrozen || frozen.FrozenAt == "" {
		return fmt.Errorf("version not frozen: %+v", frozen)
	}
	// 冻结后非法转移被拒绝
	if _, err := smokeApp.Version.Publish(ctx, ver.ID, model.VersionShared); err != model.ErrStateMachine {
		return fmt.Errorf("frozen republish: err = %v, want ErrStateMachine", err)
	}
	published, err := smokeApp.TransitionBatch(ctx, batch.ID, model.BatchPublished)
	if err != nil {
		return fmt.Errorf("transition batch to published: %w", err)
	}
	if published.Status != model.BatchPublished {
		return fmt.Errorf("batch status = %s, want published", published.Status)
	}
	// 派生新草稿
	derived, err := smokeApp.Version.Derive(ctx, ver.ID, "v2 derived")
	if err != nil {
		return fmt.Errorf("derive version: %w", err)
	}
	if derived.EventCount != 3 {
		return fmt.Errorf("derived event count = %d, want 3", derived.EventCount)
	}
	detail, err := smokeApp.Version.DetailOf(ctx, ver.ID)
	if err != nil {
		return fmt.Errorf("version detail: %w", err)
	}
	if len(detail.Events) != 3 {
		return fmt.Errorf("version detail events = %d, want 3", len(detail.Events))
	}

	// 10. 关闭重开：验证持久化与重启恢复
	if err := db.Close(); err != nil {
		return fmt.Errorf("close smoke db: %w", err)
	}
	restored, err := store.Open(smokeDB)
	if err != nil {
		return fmt.Errorf("reopen smoke db: %w", err)
	}
	defer restored.Close()
	app2 := service.New(restored)

	rb, err := app2.Batches.Get(ctx, batch.ID)
	if err != nil {
		return fmt.Errorf("restored batch: %w", err)
	}
	if rb.Status != model.BatchPublished || rb.FrameCount != 4 || rb.ObsCount != 19 {
		return fmt.Errorf("restored batch mismatch: status=%s frames=%d obs=%d", rb.Status, rb.FrameCount, rb.ObsCount)
	}
	rEvents, err := app2.Events.ListByBatch(ctx, batch.ID)
	if err != nil || len(rEvents) != 3 {
		return fmt.Errorf("restored events: n=%d err=%v, want 3", len(rEvents), err)
	}
	rv, err := app2.Version.Get(ctx, ver.ID)
	if err != nil || rv.Status != model.VersionSuperseded || rv.SupersededBy == "" {
		return fmt.Errorf("restored version: %v status=%s superseded_by=%s", err, versionStatusOf(rv), versionSupersededByOf(rv))
	}
	rDerived, err := app2.Version.Get(ctx, derived.ID)
	if err != nil || rDerived.Status != model.VersionDraft || rDerived.EventCount != 3 {
		return fmt.Errorf("restored derived: %v status=%s events=%d", err, versionStatusOf(rDerived), eventCountOf(rDerived))
	}
	// 幂等续写：重启后可继续导入新帧
	if _, created, err := app2.Ingest.ImportFrame(ctx, batch.ID, 5, 132, "ch1", "img-5"); err != nil || !created {
		return fmt.Errorf("ingest after restart: created=%v err=%v", created, err)
	}
	return nil
}

// verdictDecide 组装裁决请求（smoke 用）。
func verdictDecide(id string, confirm bool) verdict.Decision {
	return verdict.Decision{EventID: id, Confirm: confirm}
}

// eventKindOf 提取事件列表的 kind 摘要。
func eventKindOf(events []*model.LineageEvent) string {
	if len(events) == 0 {
		return "<none>"
	}
	return events[0].Kind
}

// versionStatusOf 安全取版本状态。
func versionStatusOf(v *model.Version) string {
	if v == nil {
		return "<nil>"
	}
	return v.Status
}

// versionSupersededByOf 安全取版本替代链。
func versionSupersededByOf(v *model.Version) string {
	if v == nil {
		return "<nil>"
	}
	return v.SupersededBy
}

// eventCountOf 安全取版本事件数。
func eventCountOf(v *model.Version) int {
	if v == nil {
		return -1
	}
	return v.EventCount
}

// mustGetObs 读取观测，失败即 panic（smoke 场景内不允许查询失败）。
func mustGetObs(app *service.App, ctx context.Context, id string) *model.Observation {
	o, err := app.Obs.Get(ctx, id)
	if err != nil {
		panic(fmt.Sprintf("mustGetObs %s: %v", id, err))
	}
	return o
}
