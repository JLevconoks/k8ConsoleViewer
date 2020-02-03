package terminal

import (
	"bytes"
	"os"
	"os/exec"
	"text/template"
)

const itermTemplate = `import AppKit
import iterm2
bundle = "com.googlecode.iterm2"

async def main(connection):
    if not AppKit.NSRunningApplication.runningApplicationsWithBundleIdentifier_(bundle):
        AppKit.NSWorkspace.sharedWorkspace().launchApplication_("iTerm")

    app = await iterm2.async_get_app(connection)

    w = await iterm2.Window.async_create(connection)
    window = app.current_window
    await window.async_activate()
    domain = iterm2.broadcast.BroadcastDomain()

    {{range $index, $command := . -}}
    {{ if eq $index 0 -}}
    left0 = window.current_tab.current_session
    domain.add_session(left0)
    await left0.async_send_text("{{$command}}\n")
    {{else if eq $index 1 -}}
    right0 = await left0.async_split_pane(vertical=True)
    domain.add_session(right0)
    await right0.async_send_text("{{$command}}\n")
    {{else -}}
    {{ $leftRight := remainder $index 2 -}}
    {{ $winIndex := div $index 2 -}}
    {{ $parentIndex := minus $winIndex 1 -}}

    {{if eq $leftRight 0 -}}
    left{{$winIndex}} = await left{{$parentIndex}}.async_split_pane()
    domain.add_session(left{{$winIndex}})
    await left{{$winIndex}}.async_send_text("{{$command}}\n")
    {{else -}}
    right{{$winIndex}} = await right{{$parentIndex}}.async_split_pane()
    domain.add_session(right{{$winIndex}})
    await right{{$winIndex}}.async_send_text("{{$command}}\n")
    {{end -}}
    {{end -}}
    {{end}}
    await iterm2.async_set_broadcast_domains(connection, [domain])

iterm2.run_until_complete(main)`

func OpenAndExecute(commands []string) error {
	fm := template.FuncMap{
		"remainder": func(i, j int) int { return i % j },
		"div":       func(i, j int) int { return i / j },
		"minus":     func(i, j int) int { return i - j },
	}
	templ := template.New("openIterm.template").Funcs(fm)

	templ, err := templ.Parse(itermTemplate)
	if err != nil {
		return err
	}

	template.Must(templ, err)
	var b bytes.Buffer

	err = templ.Execute(&b, commands)
	if err != nil {
		return err
	}

	cmd := exec.Command("python3", "-c", b.String())

	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stdout

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
