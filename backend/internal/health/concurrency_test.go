package health

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestCollectHealthChecksRunsIndependently(t *testing.T) {
	started := make(chan string, 3)
	release := make(chan struct{})
	tasks := []healthTask{}
	for _, name := range []string{"postgres", "redis", "elasticsearch"} {
		name := name
		tasks = append(tasks, healthTask{
			name: name,
			check: func(ctx context.Context) ServiceHealth {
				started <- name
				select {
				case <-release:
					return ServiceHealth{Status: StatusHealthy, Message: name}
				case <-ctx.Done():
					return ServiceHealth{Status: StatusUnhealthy}
				}
			},
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result := make(chan map[string]ServiceHealth, 1)
	go func() { result <- collectHealthChecks(ctx, tasks) }()
	for i := 0; i < len(tasks); i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("probes executed serially or deadlocked")
		}
	}
	close(release)
	select {
	case values := <-result:
		if len(values) != len(tasks) {
			t.Fatalf("got %d checks, want %d", len(values), len(tasks))
		}
		for _, task := range tasks {
			if values[task.name].Status != StatusHealthy || values[task.name].Message != task.name {
				t.Fatalf("unexpected result for %s: %#v", task.name, values[task.name])
			}
		}
	case <-time.After(time.Second):
		t.Fatal("health check collector never returned")
	}
}

func TestCheckRedisParallelProbesDoNotOverwrite(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()
	checker := New(nil, client)
	const count = 30
	var wg sync.WaitGroup
	errs := make(chan ServiceHealth, count)
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			status := checker.CheckRedis(ctx)
			if status.Status != StatusHealthy {
				errs <- status
			}
		}()
	}
	wg.Wait()
	close(errs)
	for problem := range errs {
		t.Errorf("parallel probe returned %s: %s", problem.Status, problem.Message)
	}
}

func TestCollectHealthChecksWithNoTasks(t *testing.T) {
	checks := collectHealthChecks(context.Background(), nil)
	if len(checks) != 0 || OverallFromChecks(checks) != StatusUnhealthy {
		t.Fatalf("empty probes should fail safely: %#v", checks)
	}
}
