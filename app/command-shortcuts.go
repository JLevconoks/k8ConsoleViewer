package app

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"text/template"
)

type ClipboardShortcut struct {
	name        string
	rawTemplate string
	template    *template.Template
}

type GuiItemInfo struct {
	itemType  Type
	Context   string
	Namespace string
	Group     string
	Pod       string
	Container string
}

func defaultClipboardShortcuts() (map[Type]map[rune]ClipboardShortcut, error) {
	m := make(map[Type]map[rune]ClipboardShortcut)
	ns := make(map[rune]ClipboardShortcut)
	ns['1'] = ClipboardShortcut{name: "get all", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} get all"}
	ns['2'] = ClipboardShortcut{name: "ingress", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} get ingress"}
	ns['3'] = ClipboardShortcut{name: "events", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} get ev --sort-by=.lastTimestamp"}
	ns['4'] = ClipboardShortcut{name: "describe", rawTemplate: "kubectl --context {{.Context}} describe ns {{.Namespace}}"}
	ns['5'] = ClipboardShortcut{name: "secrets", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} get secrets"}
	ns['6'] = ClipboardShortcut{name: "cm", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} get cm"}
	m[TypeNamespace] = ns

	pg := make(map[rune]ClipboardShortcut)
	pg['1'] = ClipboardShortcut{name: "describe", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} describe deployment {{.Group}}"}
	pg['2'] = ClipboardShortcut{name: "delete", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} delete deployment {{.Group}}"}
	pg['3'] = ClipboardShortcut{name: "scale", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} scale deployment {{.Group}} --replicas="}
	m[TypePodGroup] = pg

	pod := make(map[rune]ClipboardShortcut)
	pod['1'] = ClipboardShortcut{name: "logs", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} logs {{.Pod}}"}
	pod['2'] = ClipboardShortcut{name: "exec", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} exec -it {{.Pod}} /bin/bash"}
	pod['3'] = ClipboardShortcut{name: "describe", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} describe pod {{.Pod}}"}
	pod['4'] = ClipboardShortcut{name: "delete", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} delete pod {{.Pod}}"}
	m[TypePod] = pod

	cont := make(map[rune]ClipboardShortcut)
	cont['1'] = ClipboardShortcut{name: "logs", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} logs {{.Pod}} -c {{.Container}}"}
	cont['2'] = ClipboardShortcut{name: "exec", rawTemplate: "kubectl --context {{.Context}} -n {{.Namespace}} exec -it {{.Pod}} -c {{.Container}} /bin/bash"}
	m[TypeContainer] = cont

	return parseTemplates(m)
}

func parseTemplates(m map[Type]map[rune]ClipboardShortcut) (map[Type]map[rune]ClipboardShortcut, error) {
	for typeKey, shortcuts := range m {
		for runeKey, shortcut := range shortcuts {
			shortcut, err := shortcut.parseTemplate(typeKey)
			if err != nil {
				return nil, err
			}
			shortcuts[runeKey] = shortcut
		}
	}

	return m, nil
}

func (cs ClipboardShortcut) parseTemplate(t Type) (ClipboardShortcut, error) {
	tmpl := template.New(t.String() + cs.name)
	tmpl, err := tmpl.Parse(cs.rawTemplate)
	if err != nil {
		return ClipboardShortcut{}, err
	}
	cs.template = tmpl

	return cs, nil
}

// This is a manual way of converting map[string]interface{} from viper settings to map[Type]map[rune]ClipboardShortcut
// One day I will figure out how to Unmarshal with custom Types as keys.
func convertFromViperSettings(settings map[string]interface{}) (map[Type]map[rune]ClipboardShortcut, error) {
	clipbSh, ok := settings["clipboardshortcuts"]
	if !ok {
		return nil, nil
	}
	ret := make(map[Type]map[rune]ClipboardShortcut)

	for typeName, shortcuts := range clipbSh.(map[string]interface{}) {
		t, err := toType(typeName)
		if err != nil {
			return nil, err
		}
		scMap := make(map[rune]ClipboardShortcut)

		for key, values := range shortcuts.(map[string]interface{}) {
			if len(key) != 1 {
				return nil, errors.New(fmt.Sprintf("Error in shortcut key value '%v'", key))
			}

			v := values.(map[string]interface{})
			intName, ok := v["name"]
			name := intName.(string)
			if !ok || len(name) == 0 {
				return nil, errors.New(fmt.Sprintf("Name not provided for clipboard shortcut '%v' for '%v'", key, typeName))
			}
			intTmpl, ok := v["template"]
			tmpl := intTmpl.(string)
			if !ok || len(tmpl) == 0 {
				return nil, errors.New(fmt.Sprintf("Template not provided for clipboard shortcut '%v' for '%v'", key, typeName))
			}

			scMap[rune(key[0])] = ClipboardShortcut{name: name, rawTemplate: tmpl}
		}

		ret[t] = scMap
	}

	return parseTemplates(ret)
}

func getShortcutDisplayMap(shortcutMap map[Type]map[rune]ClipboardShortcut) map[Type][]string {
	result := make(map[Type][]string)

	for typeKey, shortcuts := range shortcutMap {
		// sort keys
		keys := make([]rune, 0, len(shortcuts))
		for key := range shortcuts {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] < keys[j]
		})
		var line1, line2 string
		for index, value := range keys {

			sc := shortcuts[value]
			if index%2 == 0 {
				line1 = fmt.Sprintf("%v%v: %v\t", line1, string(value), sc.name)
			} else {
				line2 = fmt.Sprintf("%v%v: %v\t", line2, string(value), sc.name)
			}
		}
		buf := new(bytes.Buffer)
		tw := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, line1)
		_, _ = fmt.Fprintln(tw, line2)
		_ = tw.Flush()
		tmpLine := buf.String()
		split := strings.Split(tmpLine, "\n")
		result[typeKey] = []string{split[0], split[1]}
	}
	return result
}
