package evaluator

var registry = map[string]Operator{}

// Register adds an operator under its own Name(). Built-in operators register
// themselves from this package's init() below; new operators can be added from
// anywhere without editing Evaluate.
func Register(op Operator) {
	registry[op.Name()] = op
}

// Get resolves an operator by name.
func Get(name string) (Operator, bool) {
	op, ok := registry[name]
	return op, ok
}

func init() {
	Register(equalsOperator{})
	Register(notEqualsOperator{})
	Register(containsOperator{})
	Register(existsOperator{})
	Register(notContainsOperator{})
	Register(startsWithOperator{})
	Register(endsWithOperator{})
	Register(regexOperator{})
	Register(inOperator{})
	Register(notInOperator{})
	Register(greaterThanOperator{})
	Register(lessThanOperator{})
	Register(greaterThanOrEqualOperator{})
	Register(lessThanOrEqualOperator{})
}
