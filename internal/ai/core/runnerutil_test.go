package core

import (
	"context"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

func TestCollectEventsUsesRunnerCompletionChoicesWhenNoVisibleContent(t *testing.T) {
	ch := make(chan *event.Event, 1)
	ch <- &event.Event{Response: &trpcmodel.Response{
		Object: trpcmodel.ObjectTypeRunnerCompletion,
		Done:   true,
		Choices: []trpcmodel.Choice{{
			Message: trpcmodel.NewAssistantMessage("最终答案"),
		}},
	}}
	close(ch)

	got, err := CollectEvents(context.Background(), ch)
	if err != nil {
		t.Fatalf("CollectEvents err = %v", err)
	}
	if got != "最终答案" {
		t.Fatalf("CollectEvents = %q, want 最终答案", got)
	}
}

func TestCollectEventsUsesRunnerCompletionStateDeltaWhenNoChoices(t *testing.T) {
	ch := make(chan *event.Event, 1)
	ch <- &event.Event{Response: &trpcmodel.Response{
		Object: trpcmodel.ObjectTypeRunnerCompletion,
		Done:   true,
	}, StateDelta: map[string][]byte{
		graph.StateKeyLastResponse: []byte(`"state 最终答案"`),
	}}
	close(ch)

	got, err := CollectEvents(context.Background(), ch)
	if err != nil {
		t.Fatalf("CollectEvents err = %v", err)
	}
	if got != "state 最终答案" {
		t.Fatalf("CollectEvents = %q, want state 最终答案", got)
	}
}

func TestCollectEventsDoesNotDuplicateRunnerCompletionChoices(t *testing.T) {
	ch := make(chan *event.Event, 2)
	ch <- &event.Event{Response: &trpcmodel.Response{
		Object: trpcmodel.ObjectTypeChatCompletion,
		Choices: []trpcmodel.Choice{{
			Message: trpcmodel.NewAssistantMessage("最终答案"),
		}},
	}}
	ch <- &event.Event{Response: &trpcmodel.Response{
		Object: trpcmodel.ObjectTypeRunnerCompletion,
		Done:   true,
		Choices: []trpcmodel.Choice{{
			Message: trpcmodel.NewAssistantMessage("最终答案"),
		}},
	}}
	close(ch)

	got, err := CollectEvents(context.Background(), ch)
	if err != nil {
		t.Fatalf("CollectEvents err = %v", err)
	}
	if got != "最终答案" {
		t.Fatalf("CollectEvents = %q, want 最终答案", got)
	}
}
