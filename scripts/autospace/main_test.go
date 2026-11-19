package main

import "testing"

func TestSpace(t *testing.T) {
	const th = " "
	cases := []struct{ in, want string }{
		{"Agent 访问", "Agent" + th + "访问"},
		{"共 {total} 条", "共" + th + "{total}" + th + "条"},
		{"按住{mod}显示快捷键", "按住" + th + "{mod}" + th + "显示快捷键"},
		{"{count, plural, other {# 条匹配}} · 共 {total} 条", "{count, plural, other {#" + th + "条匹配}} · 共" + th + "{total}" + th + "条"},
		{"7 天内", "7" + th + "天内"},
		{"保留 Tab、空格", "保留" + th + "Tab、空格"},
		{"{count, plural, =0 {暂无版本} other {# 个版本}}", "{count, plural, =0 {暂无版本} other {#" + th + "个版本}}"},
		{"「{title}」已删除", "「{title}」已删除"}, // punctuation, not Han: no space
		{"已经 正确", "已经 正确"},               // Han and Han: unchanged
		{"Copy all", "Copy all"},
	}
	for _, c := range cases {
		if got := Space(c.in); got != c.want {
			t.Errorf("Space(%q) = %q; want %q", c.in, got, c.want)
		}
		if again := Space(Space(c.in)); again != Space(c.in) {
			t.Errorf("not idempotent on %q", c.in)
		}
	}
}
