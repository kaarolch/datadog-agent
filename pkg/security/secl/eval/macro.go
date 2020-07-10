package eval

import (
	"github.com/DataDog/datadog-agent/pkg/security/secl/ast"
	"github.com/pkg/errors"
)

// Macro - macro object identified by an `ID` containing a SECL `Expression`
type Macro struct {
	ID         MacroID
	Expression string
	Opts       *Opts

	evaluator *MacroEvaluator
	ast       *ast.Macro
}

type MacroEvaluator struct {
	Value       interface{}
	EventTypes  []EventType
	FieldValues map[Field][]FieldValue

	partialEvals map[Field]func(ctx *Context) bool
}

// GetEvaluator returns the MacroEvaluator of the Macro corresponding to the SECL `Expression`
func (m *Macro) GetEvaluator() *MacroEvaluator {
	return m.evaluator
}

// GetAst returns the representation of the SECL `Expression`
func (m *Macro) GetAst() *ast.Macro {
	return m.ast
}

// Parse transforms the SECL `Expression` into its AST representation
func (m *Macro) Parse() error {
	astMacro, err := ast.ParseMacro(m.Expression)
	if err != nil {
		return err
	}
	m.ast = astMacro
	return nil
}

func macroToEvaluator(macro *ast.Macro, model Model, opts *Opts, field Field) (*MacroEvaluator, error) {
	macros := make(map[MacroID]*MacroEvaluator)
	for id, macro := range opts.Macros {
		macros[id] = macro.evaluator
	}
	state := newState(model, field, macros)

	var eval interface{}
	var err error

	switch {
	case macro.Expression != nil:
		eval, _, _, err = nodeToEvaluator(macro.Expression, opts, state)
	case macro.Array != nil:
		eval, _, _, err = nodeToEvaluator(macro.Array, opts, state)
	case macro.Primary != nil:
		eval, _, _, err = nodeToEvaluator(macro.Primary, opts, state)
	}

	if err != nil {
		return nil, err
	}

	events, err := eventFromFields(model, state)
	if err != nil {
		return nil, err
	}

	return &MacroEvaluator{
		Value:       eval,
		EventTypes:  events,
		FieldValues: state.fieldValues,
	}, nil
}

func (m *Macro) GenEvaluator(model Model, opts *Opts) error {
	m.Opts = opts

	evaluator, err := macroToEvaluator(m.ast, model, opts, "")
	if err != nil {
		if err, ok := err.(*AstToEvalError); ok {
			return errors.Wrap(&RuleParseError{pos: err.Pos, expr: m.Expression}, "macro syntax error")
		}
		return errors.Wrap(err, "macro compilation error")
	}
	m.evaluator = evaluator

	return nil
}

// GetEventTypes returns a list of all the event that the `Expression` handles
func (m *Macro) GetEventTypes() []EventType {
	eventTypes := m.evaluator.EventTypes

	for _, macro := range m.Opts.Macros {
		eventTypes = append(eventTypes, macro.GetEventTypes()...)
	}

	return eventTypes
}

func (m *MacroEvaluator) GetFields() []Field {
	fields := make([]Field, len(m.FieldValues))
	i := 0
	for key := range m.FieldValues {
		fields[i] = key
		i++
	}
	return fields
}

func (m *Macro) GetFields() []Field {
	fields := m.evaluator.GetFields()

	for _, macro := range m.Opts.Macros {
		fields = append(fields, macro.GetFields()...)
	}

	return fields
}