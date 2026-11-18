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
	visited := map[string]bool{}
	var walk func(parse.Node, bool) error
	check := func(path []string) error {
		if len(path) >= 2 && path[0] == "formData" && !keys[path[1]] {
			return fmt.Errorf("%w: unknown form field %q", approval.ErrInvalidTitleTemplate, path[1])
		}
		return nil
	}
	walk = func(n parse.Node, root bool) error {
		switch x := n.(type) {
		case *parse.FieldNode:
			if root {
				return check(x.Ident)
			}
		case *parse.VariableNode:
			if x.Ident[0] == "$" {
				return check(x.Ident[1:])
			}
		case *parse.ListNode:
			if x != nil {
				for _, child := range x.Nodes {
					if err := walk(child, root); err != nil {
						return err
					}
				}
			}
		case *parse.ActionNode:
			return walk(x.Pipe, root)
		case *parse.PipeNode:
			if x != nil {
				for _, cmd := range x.Cmds {
					if err := walk(cmd, root); err != nil {
						return err
					}
				}
			}
		case *parse.CommandNode:
			for _, arg := range x.Args {
				if err := walk(arg, root); err != nil {
					return err
				}
			}
		case *parse.ChainNode:
			return walk(x.Node, root)
		case *parse.IfNode:
			return walk(&x.BranchNode, root)
		case *parse.RangeNode:
			return walk(&x.BranchNode, root)
		case *parse.WithNode:
			return walk(&x.BranchNode, root)
		case *parse.BranchNode:
			if err := walk(x.Pipe, root); err != nil {
				return err
			}
			if err := walk(x.List, root && (x.Type() == parse.NodeIf || (x.Type() == parse.NodeWith && x.Pipe.String() == "."))); err != nil {
				return err
			}
			return walk(x.ElseList, root)
		case *parse.TemplateNode:
			if err := walk(x.Pipe, root); err != nil {
				return err
			}
			if x.Pipe != nil && (x.Pipe.String() == "$" || (x.Pipe.String() == "." && root)) && !visited[x.Name] {
				visited[x.Name] = true
				if sub := tmpl.Lookup(x.Name); sub != nil {
					return walk(sub.Root, true)
				}
			}
		}
		return nil
	}
	return walk(tmpl.Root, true)
}
