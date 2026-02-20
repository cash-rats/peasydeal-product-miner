package source

import "testing"

func TestDetect_Shopee(t *testing.T) {
	t.Parallel()

	src, err := Detect("https://shopee.tw/product/1/2")
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if src != Shopee {
		t.Fatalf("expected %q, got %q", Shopee, src)
	}
}

func TestDetect_Taobao(t *testing.T) {
	t.Parallel()

	src, err := Detect("https://item.taobao.com/item.htm?id=123")
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if src != Taobao {
		t.Fatalf("expected %q, got %q", Taobao, src)
	}
}

func TestDetect_TmallAsTaobao(t *testing.T) {
	t.Parallel()

	cases := []string{
		"https://tmall.com/item.htm?id=1",
		"https://detail.tmall.com/item.htm?id=1",
	}
	for _, raw := range cases {
		src, err := Detect(raw)
		if err != nil {
			t.Fatalf("Detect(%q) error: %v", raw, err)
		}
		if src != Taobao {
			t.Fatalf("Detect(%q): expected %q, got %q", raw, Taobao, src)
		}
	}
}
