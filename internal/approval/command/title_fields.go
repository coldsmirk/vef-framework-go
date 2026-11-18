package command

import (
	"fmt"
	"text/template"
	"text/template/parse"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// validateTitleFields checks static references in the title's execution tree.
// Under range or with, dot belongs to the item; $ still names the title context.
func validateTitleFields(source string, fields []approval.FormFieldDefinition) error {
	if source == "" {
		return nil
	}

	tmpl, err := template.New("title").Parse(source)
	if err != nil {
		return fmt.Errorf("%w: %w", approval.ErrInvalidTitleTemplate, err)
	}

	keys := make(map[string]bool, len(fields))
	for _, f := range fields {
		keys[f.Key] = true
	}

	return (&titleFieldValidator{tmpl: tmpl, keys: keys, visited: make(map[string]bool)}).walk(tmpl.Root, true)
}

type titleFieldValidator struct {
	tmpl          *template.Template
	keys, visited map[string]bool
}

func (v *titleFieldValidator) check(path []string) error {
	if len(path) >= 2 && path[0] == "formData" && !v.keys[path[1]] {
		return fmt.Errorf("%w: unknown form field %q", approval.ErrInvalidTitleTemplate, path[1])
	}

	return nil
}
func (v *titleFieldValidator) walk(n parse.Node, root bool) error {
	switch x := n.(type) {
	case *parse.FieldNode:
		if root {
			return v.check(x.Ident)
		}
	case *parse.VariableNode:
		if x.Ident[0] == "$" {
			return v.check(x.Ident[1:])
		}
	case *parse.ListNode:
		if x != nil {
			for _, child := range x.Nodes {
				if err := v.walk(child, root); err != nil {
					return err
				}
			}
		}

	case *parse.ActionNode:
		return v.walk(x.Pipe, root)
	case *parse.PipeNode:
		if x != nil {
			for _, cmd := range x.Cmds {
				if err := v.walk(cmd, root); err != nil {
					return err
				}
			}
		}

	case *parse.CommandNode:
		for _, arg := range x.Args {
			if err := v.walk(arg, root); err != nil {
				return err
			}
		}

	case *parse.ChainNode:
		return v.walk(x.Node, root)
	case *parse.IfNode:
		return v.walk(&x.BranchNode, root)
	case *parse.RangeNode:
		return v.walk(&x.BranchNode, root)
	case *parse.WithNode:
		return v.walk(&x.BranchNode, root)
	case *parse.BranchNode:
		return v.branch(x, root)
	case *parse.TemplateNode:
		return v.named(x, root)
	}
	return nil
}

func (v *titleFieldValidator) branch(x *parse.BranchNode, root bool) error {
	if err := v.walk(x.Pipe, root); err != nil {
		return err
	}

	if err := v.walk(x.List, root && (x.Type() == parse.NodeIf || (x.Type() == parse.NodeWith && x.Pipe.String() == "."))); err != nil {
		return err
	}

	return v.walk(x.ElseList, root)

}

func (v *titleFieldValidator) named(x *parse.TemplateNode, root bool) error {
	if err := v.walk(x.Pipe, root); err != nil {
		return err
	}

	if x.Pipe != nil && (x.Pipe.String() == "$" || (x.Pipe.String() == "." && root)) && !v.visited[x.Name] {
		v.visited[x.Name] = true
		if sub := v.tmpl.Lookup(x.Name); sub != nil {
			return v.walk(sub.Root, true)
		}
	}

	return nil
}
