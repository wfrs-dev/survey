package survey

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wfrs-dev/survey/v2/core"
	"github.com/wfrs-dev/survey/v2/terminal"
)

func init() {
	// disable color output for all prompts to simplify testing
	core.DisableColor = true
}

func TestEditorRender(t *testing.T) {
	tests := []struct {
		title    string
		prompt   Editor
		data     EditorTemplateData
		expected string
	}{
		{
			"Test Editor question output without default",
			Editor{Message: "What is your favorite month:"},
			EditorTemplateData{},
			fmt.Sprintf("%s What is your favorite month: [Enter to launch editor] ", defaultIcons().Question.Text),
		},
		{
			"Test Editor question output with default",
			Editor{Message: "What is your favorite month:", Default: "April"},
			EditorTemplateData{},
			fmt.Sprintf("%s What is your favorite month: (April) [Enter to launch editor] ", defaultIcons().Question.Text),
		},
		{
			"Test Editor question output with HideDefault",
			Editor{Message: "What is your favorite month:", Default: "April", HideDefault: true},
			EditorTemplateData{},
			fmt.Sprintf("%s What is your favorite month: [Enter to launch editor] ", defaultIcons().Question.Text),
		},
		{
			"Test Editor answer output",
			Editor{Message: "What is your favorite month:"},
			EditorTemplateData{Answer: "October", ShowAnswer: true},
			fmt.Sprintf("%s What is your favorite month: October\n", defaultIcons().Question.Text),
		},
		{
			"Test Editor question output without default but with help hidden",
			Editor{Message: "What is your favorite month:", Help: "This is helpful"},
			EditorTemplateData{},
			fmt.Sprintf("%s What is your favorite month: [%s for help] [Enter to launch editor] ", defaultIcons().Question.Text, defaultPromptConfig().HelpInput),
		},
		{
			"Test Editor question output with default and with help hidden",
			Editor{Message: "What is your favorite month:", Default: "April", Help: "This is helpful"},
			EditorTemplateData{},
			fmt.Sprintf("%s What is your favorite month: [%s for help] (April) [Enter to launch editor] ", defaultIcons().Question.Text, defaultPromptConfig().HelpInput),
		},
		{
			"Test Editor question output without default but with help shown",
			Editor{Message: "What is your favorite month:", Help: "This is helpful"},
			EditorTemplateData{ShowHelp: true},
			fmt.Sprintf("%s This is helpful\n%s What is your favorite month: [Enter to launch editor] ", defaultIcons().Help.Text, defaultIcons().Question.Text),
		},
		{
			"Test Editor question output with default and with help shown",
			Editor{Message: "What is your favorite month:", Default: "April", Help: "This is helpful"},
			EditorTemplateData{ShowHelp: true},
			fmt.Sprintf("%s This is helpful\n%s What is your favorite month: (April) [Enter to launch editor] ", defaultIcons().Help.Text, defaultIcons().Question.Text),
		},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			r, w, err := os.Pipe()
			assert.NoError(t, err)

			test.prompt.WithStdio(terminal.Stdio{Out: w})
			test.data.Editor = test.prompt

			// set the icon set
			test.data.Config = defaultPromptConfig()

			err = test.prompt.Render(
				EditorQuestionTemplate,
				test.data,
			)
			assert.NoError(t, err)

			assert.NoError(t, w.Close())
			var buf bytes.Buffer
			_, err = io.Copy(&buf, r)
			assert.NoError(t, err)

			assert.Contains(t, buf.String(), test.expected)
		})
	}
}

func TestEditorPrompt(t *testing.T) {
	// The editor is a script, not a real `vi`. Depending on the system `vi` may
	// be vim, neovim or a wrapper, and driving it over the pty depends on its
	// screen updates and timing: that made this test hang instead of fail. What
	// the prompt itself owns is narrower and fully observable here: it writes the
	// initial value, runs the configured command with the temp file appended as
	// its last argument, and returns whatever the editor left in that file.
	const (
		// stands in for a user typing content and saving
		writeAnswer = `printf '%s\n' 'Add editor prompt tests' > "$file"`
		// stands in for quitting the editor without modifying the file
		leaveFile = `:`
		// stands in for deleting every line before saving
		emptyFile = `: > "$file"`
	)

	editorWithAnswer := fakeEditor(t, writeAnswer)
	editorLeavingFile := fakeEditor(t, leaveFile)
	editorEmptyingFile := fakeEditor(t, emptyFile)

	tests := []PromptTest{
		{
			"Test Editor prompt interaction",
			&Editor{
				Editor:  editorWithAnswer,
				Message: "Edit git commit message",
			},
			func(c expectConsole) {
				c.ExpectString("Edit git commit message [Enter to launch editor]")
				c.SendLine("")
				c.ExpectEOF()
			},
			"Add editor prompt tests\n",
		},
		{
			"Test Editor prompt interaction with default",
			&Editor{
				Editor:  editorLeavingFile,
				Message: "Edit git commit message",
				Default: "No comment",
			},
			func(c expectConsole) {
				c.ExpectString("Edit git commit message (No comment) [Enter to launch editor]")
				c.SendLine("")
				c.ExpectEOF()
			},
			"No comment",
		},
		{
			"Test Editor prompt interaction overriding default",
			&Editor{
				Editor:  editorWithAnswer,
				Message: "Edit git commit message",
				Default: "No comment",
			},
			func(c expectConsole) {
				c.ExpectString("Edit git commit message (No comment) [Enter to launch editor]")
				c.SendLine("")
				c.ExpectEOF()
			},
			"Add editor prompt tests\n",
		},
		{
			"Test Editor prompt interaction hiding default",
			&Editor{
				Editor:      editorLeavingFile,
				Message:     "Edit git commit message",
				Default:     "No comment",
				HideDefault: true,
			},
			func(c expectConsole) {
				c.ExpectString("Edit git commit message [Enter to launch editor]")
				c.SendLine("")
				c.ExpectEOF()
			},
			"No comment",
		},
		{
			"Test Editor prompt interaction and prompt for help",
			&Editor{
				Editor:  editorWithAnswer,
				Message: "Edit git commit message",
				Help:    "Describe your git commit",
			},
			func(c expectConsole) {
				c.ExpectString(
					fmt.Sprintf(
						"Edit git commit message [%s for help] [Enter to launch editor]",
						defaultPromptConfig().HelpInput,
					),
				)
				c.SendLine("?")
				c.ExpectString("Describe your git commit")
				c.SendLine("")
				c.ExpectEOF()
			},
			"Add editor prompt tests\n",
		},
		{
			"Test Editor prompt interaction with default and append default",
			&Editor{
				Editor:        editorEmptyingFile,
				Message:       "Edit git commit message",
				Default:       "No comment",
				AppendDefault: true,
			},
			func(c expectConsole) {
				c.ExpectString("Edit git commit message (No comment) [Enter to launch editor]")
				c.SendLine("")
				c.ExpectEOF()
			},
			"",
		},
		{
			"Test Editor prompt interaction with editor args",
			&Editor{
				Editor:  editorWithAnswer + " --",
				Message: "Edit git commit message",
			},
			func(c expectConsole) {
				c.ExpectString("Edit git commit message [Enter to launch editor]")
				c.SendLine("")
				c.ExpectEOF()
			},
			"Add editor prompt tests\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			RunPromptTest(t, test)
		})
	}
}

// The configured editor arguments must reach the command: survey appends the
// temp file after them, so dropping the args silently changes what the editor
// receives.
func TestEditorForwardsConfiguredArgs(t *testing.T) {
	// $1 is the configured sentinel, $2 the temp file survey appended.
	script := fakeEditor(t, `printf '%s' "$1" > "$2"`)

	RunPromptTest(t, PromptTest{
		"Editor prompt forwards configured editor args",
		&Editor{Editor: script + " --sentinel", Message: "Edit git commit message"},
		func(c expectConsole) {
			c.ExpectString("Edit git commit message [Enter to launch editor]")
			c.SendLine("")
			c.ExpectEOF()
		},
		"--sentinel",
	})
}

// AppendDefault writes the default into the temp file before launching the
// editor, so an editor that saves without changes returns the default.
func TestEditorAppendDefaultWritesDefault(t *testing.T) {
	script := fakeEditor(t, `:`)

	RunPromptTest(t, PromptTest{
		"Editor prompt with AppendDefault keeps the written default",
		&Editor{
			Editor:        script,
			Message:       "Edit git commit message",
			Default:       "No comment",
			AppendDefault: true,
		},
		func(c expectConsole) {
			c.ExpectString("Edit git commit message (No comment) [Enter to launch editor]")
			c.SendLine("")
			c.ExpectEOF()
		},
		"No comment",
	})
}

// A prompt configured with its own editor must not change the package default
// that every other prompt falls back to. Mutating it leaks the choice into
// later prompts and races with prompts running concurrently.
func TestEditorDoesNotMutateDefaultEditor(t *testing.T) {
	before := editor
	script := fakeEditor(t, `:`)

	var answer string
	RunTest(t,
		func(c expectConsole) {
			c.ExpectString("Edit git commit message [Enter to launch editor]")
			c.SendLine("")
			c.ExpectEOF()
		},
		func(stdio terminal.Stdio) error {
			prompt := &Editor{Editor: script, Message: "Edit git commit message"}
			prompt.WithStdio(stdio)

			val, err := prompt.Prompt(defaultPromptConfig())
			answer, _ = val.(string)

			return err
		},
	)

	if answer != "" {
		t.Errorf("answer = %q, want empty", answer)
	}
	if editor != before {
		t.Errorf("package default editor mutated to %q, want %q", editor, before)
	}
}

// fakeEditor writes a deterministic stand-in for a real editor and returns its
// path. Like a real editor invocation, the script receives the file survey
// created as its last argument, so bodies can operate on "$file" regardless of
// how many extra arguments the prompt config passed.
func fakeEditor(t *testing.T, body string) string {
	t.Helper()

	script := "#!/bin/sh\nset -e\nfile=\nfor arg in \"$@\"; do file=\"$arg\"; done\n" + body + "\n"

	path := filepath.Join(t.TempDir(), "fake-editor.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake editor script: %v", err)
	}

	return path
}
