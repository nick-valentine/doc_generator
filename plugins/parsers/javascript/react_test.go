package main

import (
	"doc_generator/pkg/store"
	"testing"
)

func TestJavascriptParser_ReactComponents(t *testing.T) {
	src := `
import React from 'react';

export const Button = ({label}) => {
    return <button className="btn">{label}</button>;
};

export const Card = ({title, children}) => {
    return (
        <div className="card">
            <h3>{title}</h3>
            {children}
            <Button label="Click me" />
        </div>
    );
};

class App extends React.Component {
    render() {
        return (
            <div className="app">
                <Card title="Welcome">
                    <p>React app content</p>
                </Card>
            </div>
        );
    }
}
`
	jp := &JavascriptParser{}
	s := &store.Source{}
	
	err := jp.Parse("App.jsx", []byte(src), s)
	if err != nil {
		t.Fatalf("Failed to parse react file: %v", err)
	}

	// 1. Verify Symbols
	syms := s.Symbols
	t.Logf("Found %d symbols total", len(syms))
	
	var foundButton, foundCard, foundApp bool
	for _, sym := range syms {
		t.Logf("- Symbol: %s, Kind: %s, Parent: %q", sym.Name, sym.Kind, sym.Parent)
		if sym.Name == "Button" && sym.Kind == store.SymStruct { foundButton = true }
		if sym.Name == "Card" && sym.Kind == store.SymStruct { foundCard = true }
		if sym.Name == "App" && sym.Kind == store.SymStruct { foundApp = true }
	}

	if !foundButton { t.Error("Failed to detect functional component 'Button' (Struct)") }
	if !foundCard { t.Error("Failed to detect functional component 'Card' (Struct)") }
	if !foundApp { t.Error("Failed to detect class component 'App' (Struct)") }

	// 2. Verify Functional Call Graph (Card calls Button via JSX)
	// Package is derived from dirname, currently defaults to "frontend" if directory isn't deeply nested.
	cardCallees := s.GetCallees("Card")
	t.Logf("Card callees: %v", cardCallees)
	foundRel := false
	for _, c := range cardCallees {
		if c == "Button" { foundRel = true; break }
	}
	if !foundRel {
		t.Errorf("Expected 'Card' to call 'Button' via JSX, got: %v", cardCallees)
	}

	// 3. Verify Class Render Call Graph (App.render calls Card via JSX)
	renderCallees := s.GetCallees("App.render")
	t.Logf("App.render callees: %v", renderCallees)
	foundCardRel := false
	for _, c := range renderCallees {
		if c == "Card" { foundCardRel = true; break }
	}
	if !foundCardRel {
		t.Errorf("Expected 'App.render' to call 'Card' via JSX, got: %v", renderCallees)
	}
}
