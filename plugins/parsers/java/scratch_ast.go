package main

import (
	"fmt"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

func main() {
	code := `
public interface TestInterface {
    default void defaultMethod() {
        System.out.println("default");
    }
    void regularMethod();
}

public class Test {
    public Test() {
        this(1);
    }
    
    public Test(int x) {
        super();
    }

    public void methodOne() {
        Runnable r = () -> { System.out.println("lambda"); };
        List<String> list = new java.util.ArrayList<>();
        list.forEach(System.out::println);
    }

    public <T> T genericMethod(T in) {
        return in;
    }

    private static class Inner {
        void innerMethod() {
            System.out.println("inner");
        }
    }
    
    public record Point(int x, int y) {
        public void print() {
            System.out.println(x + "," + y);
        }
    }
}
`
	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_java.Language()))

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	printAST(tree.RootNode(), 0, []byte(code))
}

func printAST(n *tree_sitter.Node, depth int, src []byte) {
	if n == nil { return }
	indent := strings.Repeat("  ", depth)
	text := string(src[n.StartByte():n.EndByte()])
	if len(text) > 30 { text = text[:30] + "..." }
	text = strings.ReplaceAll(text, "\n", " ")
	
	fmt.Printf("%s%s: %q\n", indent, n.Kind(), text)
	for i := uint(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		fn := n.FieldNameForChild(uint32(i))
		if fn != "" {
			fmt.Printf("%s  [field: %s]\n", indent, fn)
		}
		printAST(child, depth+1, src)
	}
}
