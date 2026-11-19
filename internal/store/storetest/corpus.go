// Package storetest seeds a store with a realistic, deterministic corpus
// for benchmarks: Chinese and English notes, curl commands, JSON, code,
// logs and to-do lists, from a few bytes to about 20 KB each.
package storetest

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"time"

	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/title"
)

// Base is the corpus's "now": every snippet is modified within the year
// before it, more of them recently.
var Base = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

// Absent is a word no generated snippet contains, for worst-case queries.
const Absent = "鲸鱼"

// Corpus describes a seeded store.
type Corpus struct {
	IDs  []string // newest first
	Open int      // the first Open IDs are open tabs
}

// Seed adds n snippets to s with the given seed, the newest open tabs
// open, each with zero to five versions. It changes the store's clock and
// leaves it at Base.
func Seed(ctx context.Context, s *store.Store, n, open int, seed uint64) (Corpus, error) {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	type item struct {
		at   time.Time
		kind store.Kind
		body string
	}
	items := make([]item, n)
	for i := range items {
		// Ages skew recent: a quarter in the last week, most in 90 days.
		age := time.Duration(r.ExpFloat64() * 60 * float64(24*time.Hour))
		age = min(age, 365*24*time.Hour)
		kind, body := Body(r)
		items[i] = item{Base.Add(-age - time.Duration(i)*time.Second), kind, body}
	}
	// Insert oldest first, as they were written.
	sort.Slice(items, func(i, j int) bool { return items[i].at.Before(items[j].at) })
	c := Corpus{IDs: make([]string, n), Open: min(open, n)}
	for i, it := range items {
		t := it.at
		s.SetClock(func() time.Time { return t })
		sn, err := s.CreateSnippet(ctx, it.kind, it.body, i)
		if err != nil {
			return c, err
		}
		if err := s.SetTitle(ctx, sn.ID, title.Make(title.Kind(it.kind), it.body), store.TitleAuto); err != nil {
			return c, err
		}
		for range r.IntN(6) {
			if _, err := s.AddVersion(ctx, sn.ID, it.body, store.ReasonPaused, 1+r.IntN(20)); err != nil {
				return c, err
			}
		}
		if i < n-c.Open {
			if err := s.CloseTab(ctx, sn.ID); err != nil {
				return c, err
			}
		}
		c.IDs[n-1-i] = sn.ID
	}
	s.SetClock(func() time.Time { return Base })
	return c, nil
}

// Body returns one generated snippet.
func Body(r *rand.Rand) (store.Kind, string) {
	size := targetSize(r)
	switch p := r.IntN(100); {
	case p < 30:
		return store.KindText, grow(r, size, zhParagraph)
	case p < 50:
		return store.KindText, grow(r, size, enParagraph)
	case p < 60:
		return store.KindText, grow(r, size, curl)
	case p < 72:
		return store.KindText, jsonDoc(r, size)
	case p < 85:
		return store.KindText, grow(r, size, code)
	case p < 92:
		return store.KindText, grow(r, size, logLines)
	default:
		return store.KindTodo, grow(r, min(size, 2000), todoItems)
	}
}

// targetSize: most snippets are short, a few run to about 20 KB.
func targetSize(r *rand.Rand) int {
	switch p := r.IntN(100); {
	case p < 60:
		return 40 + r.IntN(960)
	case p < 85:
		return 1000 + r.IntN(3000)
	case p < 97:
		return 4000 + r.IntN(8000)
	default:
		return 12000 + r.IntN(8500)
	}
}

func grow(r *rand.Rand, size int, part func(*rand.Rand) string) string {
	var b strings.Builder
	for b.Len() < size {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(part(r))
	}
	return b.String()
}

func pick(r *rand.Rand, xs []string) string { return xs[r.IntN(len(xs))] }

var zhSentences = []string{
	"周会上讨论了下个版本的发版计划，大家对时间表基本没有异议。",
	"数据库迁移脚本需要在凌晨低峰期执行，执行前先做一次全量备份。",
	"客户反馈导出功能在大文件下会超时，初步怀疑是分页查询没有走索引。",
	"需求评审定在周四下午，产品会先把原型发到群里。",
	"上线后观察半小时监控，错误率没有明显上升再通知大家。",
	"回滚预案：保留上一版镜像，配置中心的开关先不要删。",
	"告警阈值调整为五分钟内连续三次失败才触发，减少夜间误报。",
	"测试环境的账号密码已经重置，新的放在团队的密码库里。",
	"这周的重点是把搜索延迟压到五十毫秒以内。",
	"和设计确认了深色模式下的对比度，按钮文字需要再亮一点。",
	"接口文档里少了分页参数的说明，补上之后再发给前端。",
	"会议纪要：一、确认里程碑；二、分配负责人；三、下周复盘。",
	"服务器磁盘使用率已经到了百分之八十五，需要清理旧日志。",
	"排查发现是缓存过期时间设置得太短，导致数据库压力突增。",
	"明天上午十点和运维同步一下证书续期的事情。",
	"新同事入职，开发环境的配置步骤整理成文档放在知识库。",
	"灰度发布先放百分之五的流量，没问题再逐步扩大。",
	"性能测试结果：平均响应时间一百二十毫秒，峰值四百毫秒。",
	"记得在发版说明里写上这次修复的三个问题。",
	"周报：完成登录重构，开始做权限模块，下周联调。",
	"中文输入法在候选窗口的位置偶尔不对，需要再确认一下。",
	"把常用的命令整理到一个片段里，省得每次都去翻聊天记录。",
	"预算申请已经提交，财务说下周三之前会有结果。",
	"线上日志里出现了大量超时，集中在凌晨两点到三点之间。",
	"代码评审意见：变量命名再清晰一些，错误处理不要吞掉。",
}

var zhHeads = []string{"周会记录", "发版检查", "排查笔记", "会议纪要", "待跟进", "临时记一下", "数据库迁移", "需求评审"}

func zhParagraph(r *rand.Rand) string {
	var b strings.Builder
	if r.IntN(4) == 0 {
		b.WriteString(pick(r, zhHeads) + "\n")
	}
	for range 2 + r.IntN(5) {
		b.WriteString(pick(r, zhSentences))
	}
	if r.IntN(5) == 0 {
		fmt.Fprintf(&b, " 参考：https://wiki.example.com/pages/%d", r.IntN(100000))
	}
	return b.String()
}

var enSentences = []string{
	"The release checklist needs one more pass before we tag the build.",
	"Customer reported that exports time out on large accounts; the query does a full table scan.",
	"Remember to rotate the staging credentials after the demo.",
	"We agreed to ship the dark theme behind a flag and turn it on next week.",
	"Latency is fine at p50 but the p99 spikes every hour when the cron job runs.",
	"Draft reply: thanks for the detailed report, we have a fix queued for the next patch release.",
	"Meeting notes: scope is frozen, design review on Thursday, QA starts Monday.",
	"The migration adds a nullable column first, backfills it, then adds the constraint.",
	"Ask the platform team whether the new runners have more memory.",
	"Keyboard shortcuts conflict with the input method on some layouts.",
	"Idea: keep a scratch buffer per project and search across all of them.",
	"The flaky test only fails when the machine is under load, probably a timing issue.",
	"Budget for the offsite is approved; book the venue before Friday.",
	"Postmortem: the alert fired but nobody was on call because the rotation was empty.",
	"Use the read replica for reports so the primary stays responsive.",
	"TODO later: write down how the cache invalidation actually works.",
}

func enParagraph(r *rand.Rand) string {
	var b strings.Builder
	for i := range 2 + r.IntN(5) {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(pick(r, enSentences))
	}
	return b.String()
}

var (
	hosts     = []string{"api.example.com", "staging.internal.example.net", "localhost:8080", "gateway.example.io"}
	endpoints = []string{"/v2/users", "/v2/orders", "/v1/search", "/v2/snippets", "/internal/metrics", "/v2/webhooks/events", "/healthz"}
	jsonKeys  = []string{"id", "name", "status", "created_at", "items", "user_id", "amount", "currency", "tags", "metadata", "enabled", "retries", "region", "message"}
)

func curl(r *rand.Rand) string {
	var b strings.Builder
	method := pick(r, []string{"GET", "POST", "PUT", "DELETE"})
	fmt.Fprintf(&b, "curl -sS -X %s 'https://%s%s", method, pick(r, hosts), pick(r, endpoints))
	if r.IntN(2) == 0 {
		fmt.Fprintf(&b, "?limit=%d&page=%d", 10+r.IntN(90), r.IntN(20))
	}
	b.WriteString("' \\\n")
	fmt.Fprintf(&b, "  -H 'Authorization: Bearer %s' \\\n", token(r))
	b.WriteString("  -H 'Content-Type: application/json'")
	if r.IntN(3) == 0 {
		fmt.Fprintf(&b, " \\\n  --since %d-%02d-%02d", 2026, 1+r.IntN(11), 1+r.IntN(28))
	}
	if method == "POST" || method == "PUT" {
		fmt.Fprintf(&b, " \\\n  -d '%s'", jsonObject(r, 1))
	}
	b.WriteString("\n")
	return b.String()
}

func token(r *rand.Rand) string {
	const alnum = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	t := make([]byte, 32)
	for i := range t {
		t[i] = alnum[r.IntN(len(alnum))]
	}
	return "sk_test_" + string(t)
}

func jsonValue(r *rand.Rand, depth int) string {
	switch p := r.IntN(10); {
	case p < 3:
		return fmt.Sprint(r.IntN(100000))
	case p < 6:
		return fmt.Sprintf("%q", pick(r, []string{"active", "pending", "失败", "已完成", "cn-north-1", "us-east-1", "hello world"}))
	case p < 7:
		return pick(r, []string{"true", "false", "null"})
	case p < 9 && depth < 3:
		return jsonObject(r, depth+1)
	default:
		return fmt.Sprintf("[%d, %d, %d]", r.IntN(10), r.IntN(10), r.IntN(10))
	}
}

func jsonObject(r *rand.Rand, depth int) string {
	var b strings.Builder
	b.WriteString("{")
	for i := range 2 + r.IntN(4) {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q: %s", pick(r, jsonKeys), jsonValue(r, depth))
	}
	b.WriteString("}")
	return b.String()
}

func jsonDoc(r *rand.Rand, size int) string {
	var b strings.Builder
	b.WriteString("[\n")
	for b.Len() < size {
		if b.Len() > 2 {
			b.WriteString(",\n")
		}
		b.WriteString("  " + jsonObject(r, 0))
	}
	b.WriteString("\n]")
	return b.String()
}

var codeBlocks = []string{
	"func (s *Server) handle(w http.ResponseWriter, r *http.Request) {\n\tctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)\n\tdefer cancel()\n\tif err := s.db.PingContext(ctx); err != nil {\n\t\thttp.Error(w, err.Error(), http.StatusServiceUnavailable)\n\t\treturn\n\t}\n\tw.WriteHeader(http.StatusOK)\n}",
	"def retry(fn, attempts=3, delay=0.5):\n    for i in range(attempts):\n        try:\n            return fn()\n        except TimeoutError:\n            time.sleep(delay * 2 ** i)\n    raise RuntimeError(\"gave up\")",
	"SELECT u.id, u.name, count(o.id) AS orders\nFROM users u LEFT JOIN orders o ON o.user_id = u.id\nWHERE u.created_at >= '2026-01-01'\nGROUP BY u.id\nORDER BY orders DESC\nLIMIT 50;",
	"kubectl -n payments rollout restart deployment/api && kubectl -n payments rollout status deployment/api --timeout=120s",
	"git log --oneline --since=\"2 weeks ago\" --author=me | wc -l",
	"const res = await fetch(`/v2/users/${id}`, { headers: { Authorization: `Bearer ${token}` } });\nif (!res.ok) throw new Error(`HTTP ${res.status}`);\nconst data = await res.json();",
	"docker run --rm -it -p 5432:5432 -e POSTGRES_PASSWORD=dev postgres:17",
	"for f in *.log; do gzip -9 \"$f\"; done",
	"type Config struct {\n\tAddr    string        `json:\"addr\"`\n\tTimeout time.Duration `json:\"timeout\"`\n\tDebug   bool          `json:\"debug\"`\n}",
	"journalctl -u lastab --since \"2026-11-18 09:00\" --until \"2026-11-18 10:00\" | grep -i error",
}

func code(r *rand.Rand) string { return pick(r, codeBlocks) + "\n" }

var logMessages = []string{
	"INFO  server started addr=:8080",
	"WARN  slow query took=812ms table=orders",
	"ERROR upstream timeout after 30s host=gateway.example.io",
	"INFO  request method=GET path=/v2/users status=200 took=12ms",
	"DEBUG cache miss key=user:48213",
	"ERROR 数据库连接池耗尽 active=100 idle=0",
	"INFO  job finished name=cleanup removed=1832",
}

func logLines(r *rand.Rand) string {
	var b strings.Builder
	for range 3 + r.IntN(10) {
		fmt.Fprintf(&b, "2026-%02d-%02dT%02d:%02d:%02d.%03dZ %s\n", 1+r.IntN(11), 1+r.IntN(28),
			r.IntN(24), r.IntN(60), r.IntN(60), r.IntN(1000), pick(r, logMessages))
	}
	return b.String()
}

var todoTexts = []string{"发版前检查", "更新依赖", "写周报", "回复客户邮件", "review the migration PR", "renew TLS certificate",
	"整理会议纪要", "book flights", "跑一遍性能测试", "clean up feature flags", "确认需求变更", "back up the database"}

func todoItems(r *rand.Rand) string {
	mark := " "
	if r.IntN(3) == 0 {
		mark = "x"
	}
	return fmt.Sprintf("- [%s] %s", mark, pick(r, todoTexts))
}
