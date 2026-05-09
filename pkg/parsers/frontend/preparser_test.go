package frontend

import (
	"strings"
	"testing"
	"doc_generator/pkg/store"
)

func TestPreprocess(t *testing.T) {
	input := `
import React from 'react';

export interface MyProps {
    name: string;
    onClick: () => void;
}

type OtherType = {
    x: number;
};

export class Component extends React.Component<MyProps> {
    public myField: number = 10;
    
    render(): JSX.Element {
        return <div>Hello {this.props.name as string}</div>;
    }
}

export const Func = (arg: string): void => {
    const x = arg as string;
};
`
	syms := &store.Source{}
	output := Preprocess([]byte(input), "test.tsx", syms)
	
	outStr := string(output)
	t.Logf("CLEANED SOURCE:\n%s", outStr)

	// Check interface extraction
	foundProps := false
	foundOther := false
	for _, s := range syms.Symbols {
		if s.Name == "MyProps" && s.Kind == store.SymInterface { foundProps = true }
		if s.Name == "OtherType" && s.Kind == store.SymInterface { foundOther = true }
	}

	if !foundProps { t.Errorf("Failed to extract MyProps interface") }
	if !foundOther { t.Errorf("Failed to extract OtherType") }

	// Check strings removal
	if strings.Contains(outStr, "interface MyProps") {
		t.Errorf("Output still contains interface declaration")
	}
	if strings.Contains(outStr, "<MyProps>") {
		t.Errorf("Output still contains generic clause")
	}
	if strings.Contains(outStr, ": JSX.Element") {
		t.Errorf("Output still contains return type annotation")
	}
	if strings.Contains(outStr, "as string") {
		t.Errorf("Output still contains 'as' keyword cast")
	}
	if strings.Contains(outStr, "public") {
		t.Errorf("Output still contains public modifier")
	}
}

func TestJSXExtraction(t *testing.T) {
	src := `
	const MyDashboard = () => {
		return (
			<div>
				<Sidebar show={true} />
				<ContentArea>
					<Widget id="1" />
					<Footer />
				</ContentArea>
			</div>
		);
	}
	`
	syms := &store.Source{}
	ExtractJSXCalls("MyDashboard", src, syms)

	if len(syms.Calls) != 4 {
		t.Errorf("Expected 4 component calls, got %d", len(syms.Calls))
	}

	expected := map[string]bool{
		"Sidebar": true,
		"ContentArea": true,
		"Widget": true,
		"Footer": true,
	}

	for _, c := range syms.Calls {
		if c.Caller != "MyDashboard" {
			t.Errorf("Invalid caller name: %s", c.Caller)
		}
		delete(expected, c.Callee)
	}

	if len(expected) > 0 {
		t.Errorf("Failed to find expected subcomponents: %v", expected)
	}
}
