package watcher

// func TestErrorNotFound(t *testing.T) {
// 	a := &ErrorNotFound{Cluster: "cluster", Namespace: "namespace", Name: "name"}

// 	if !errors.Is(a, &ErrorNotFound{}) {
// 		t.Errorf("ErrorNotFound should be equal to ErrorNotFound")
// 	}

// 	wrapped := fmt.Errorf("wrapped: %w", a)
// 	if !errors.Is(wrapped, &ErrorNotFound{}) {
// 		t.Errorf("ErrorNotFound should be equal to ErrorNotFound")
// 	}

// 	var b *ErrorNotFound
// 	if !errors.As(wrapped, &b) {
// 		t.Errorf("ErrorNotFound should be equal to ErrorNotFound")
// 	}

// 	msg := "not found: cluster/namespace/name"
// 	if a.Error() != msg {
// 		t.Errorf("Error() should return %s, got %s", msg, a.Error())
// 	}
// }
