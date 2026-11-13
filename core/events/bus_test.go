package events

import "testing"

func TestPublishAndSubscribe(t *testing.T) {
	InitBus()
	ch := Bus.Subscribe()
	defer Bus.Unsubscribe(ch)

	Bus.Info("hello")

	ev := <-ch
	if ev.Type != Info || ev.Message != "hello" {
		t.Fatalf("事件 = %+v", ev)
	}
	if ev.ID == "" || ev.Time == 0 {
		t.Fatalf("事件缺少 ID 或时间: %+v", ev)
	}
}

func TestHistoryReplay(t *testing.T) {
	InitBus()
	Bus.Warn("before subscribe")
	ch := Bus.Subscribe()
	defer Bus.Unsubscribe(ch)

	ev := <-ch
	if ev.Type != Warn || ev.Message != "before subscribe" {
		t.Fatalf("回放事件 = %+v", ev)
	}
}

func TestHistoryLimit(t *testing.T) {
	InitBus()
	for i := 0; i < historyLimit+10; i++ {
		Bus.Info("e")
	}
	ch := Bus.Subscribe()
	defer Bus.Unsubscribe(ch)
	if len(ch) != historyLimit {
		t.Fatalf("回放条数 = %d，期望 %d", len(ch), historyLimit)
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	InitBus()
	ch := Bus.Subscribe()
	Bus.Unsubscribe(ch)
	if _, ok := <-ch; ok {
		t.Fatal("取消订阅后通道应关闭")
	}
}

func TestNilBusDoesNotPanic(t *testing.T) {
	var b *EventBus
	b.Error("error")
	b.Warn("warn")
	b.Info("info")
}
