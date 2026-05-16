package prompt

import (
	"bytes"
	"testing"
)

func TestAskUsesDefault(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	prompter := New(bytes.NewBufferString("\n"), &output)

	value, err := prompter.Ask("Project alias", "example", true)
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}
	if value != "example" {
		t.Fatalf("expected default value, got %q", value)
	}
}

func TestAskRepeatsRequiredQuestion(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	prompter := New(bytes.NewBufferString("\napi\n"), &output)

	value, err := prompter.Ask("Project alias", "", true)
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}
	if value != "api" {
		t.Fatalf("expected api, got %q", value)
	}
	if !bytes.Contains(output.Bytes(), []byte("Value required.")) {
		t.Fatalf("expected required warning, got:\n%s", output.String())
	}
}

func TestConfirm(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	prompter := New(bytes.NewBufferString("yes\n"), &output)

	ok, err := prompter.Confirm("Write registry", false)
	if err != nil {
		t.Fatalf("Confirm returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected confirmation")
	}
}
