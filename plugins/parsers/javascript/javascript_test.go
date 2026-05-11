package main

import (
	"doc_generator/pkg/store"
	"testing"
)

func TestJavascriptParser_Everything(t *testing.T) {
	srcCode := []byte(`
/**
 * Standard JS class
 */
class BaseHandler extends Library.Abstract {
    constructor() {
        super();
        this.setup();
    }

    setup() {
        internalLog();
    }
}

function processItems(items) {
    if (items.length > 0) {
        items.map(i => log(i));
    }
}

const heavyHelper = (a, b) => {
    if (a) return b;
    return a + b;
}

async function fetchExternalData() {
    const data = await api.call();
    return data;
}

class AsyncManager {
    async init() {
        await this.load();
    }
}
`)

	jp := &JavascriptParser{}
	source := &store.Source{}
	err := jp.Parse("app.js", srcCode, source)
	if err != nil {
		t.Fatalf("failed parsing JS: %v", err)
	}

	// 0. Check Async Attributes
	fetchSyms := source.SearchSymbols("fetchExternalData")
	if len(fetchSyms) == 0 {
		t.Errorf("fetchExternalData not found")
	} else if !fetchSyms[0].IsAsync {
		t.Errorf("fetchExternalData should be Async=true")
	}

	initSyms := source.SearchSymbols("init")
	if len(initSyms) == 0 {
		t.Errorf("AsyncManager.init not found")
	} else {
		foundInit := false
		for _, s := range initSyms {
			if s.Parent == "AsyncManager" {
				foundInit = true
				if !s.IsAsync {
					t.Errorf("AsyncManager.init should be Async=true")
				}
			}
		}
		if !foundInit {
			t.Errorf("AsyncManager.init not found among init symbols")
		}
	}

	// 1. Check Class and Heritage (Inheritance)
	syms := source.SearchSymbols("BaseHandler")
	if len(syms) == 0 {
		t.Fatalf("BaseHandler not found")
	}
	handler := syms[0]
	if len(handler.Relations) == 0 {
		t.Errorf("Expected relations captured from extends, got 0")
	} else {
		// It usually gives ['Library.Abstract']
		found := false
		for _, r := range handler.Relations {
			if r == "Library.Abstract" {
				found = true
			}
		}
		if !found {
			t.Errorf("Library.Abstract not found in relations: %v", handler.Relations)
		}
	}

	// 2. Check normal function and its complexity
	procSyms := source.SearchSymbols("processItems")
	if len(procSyms) == 0 {
		t.Fatalf("processItems not found")
	}
	proc := procSyms[0]
	if proc.Complexity <= 1 {
		t.Errorf("Expected complexity > 1 for processItems with IF, got %d", proc.Complexity)
	}

	// 3. Check arrow function complexity
	arrowSyms := source.SearchSymbols("heavyHelper")
	if len(arrowSyms) == 0 {
		t.Fatalf("heavyHelper arrow function not found")
	}
	arr := arrowSyms[0]
	if arr.Complexity <= 1 {
		t.Errorf("Expected complexity > 1 for heavyHelper arrow, got %d", arr.Complexity)
	}

	// 4. Check internal calls
	// Setup calls internalLog
	setupCallees := source.GetCallees("BaseHandler.setup")
	foundLog := false
	for _, c := range setupCallees {
		if c == "internalLog" {
			foundLog = true
		}
	}
	if !foundLog {
		t.Errorf("internalLog call relation not found from BaseHandler.setup, got %v", setupCallees)
	}
}
