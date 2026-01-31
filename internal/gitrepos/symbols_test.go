package gitrepos

import (
	"reflect"
	"sort"
	"testing"
)

func TestExtractSymbols(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		content  string
		expected []string
	}{
		{
			name: "Go functions and types",
			ext:  "go",
			content: `package main
func MyFunc() {}
type MyStruct struct{}
type MyInterface interface{}
const MyConst = 1
var MyVar = 2
`,
			expected: []string{"MyFunc", "MyStruct", "MyInterface", "MyConst", "MyVar"},
		},
		{
			name: "Python classes and defs",
			ext:  "py",
			content: `class MyClass:
    def my_method(self):
        pass

def top_level_func():
    pass
`,
			expected: []string{"MyClass", "my_method", "top_level_func"},
		},
		{
			name: "Java classes and methods",
			ext:  "java",
			content: `public class MyClass {
    private String myField;
    public void myMethod() {}
    static int staticMethod(int x) { return x; }
}
interface MyInterface {}
enum MyEnum {}
`,
			expected: []string{"MyClass", "myMethod", "staticMethod", "MyInterface", "MyEnum"},
		},
		{
			name: "JavaScript functions and consts",
			ext:  "js",
			content: `function myFunc() {}
class MyClass {}
const myConst = () => {}
let myLet = 1
var myVar = 2
`,
			expected: []string{"myFunc", "MyClass", "myConst", "myLet", "myVar"},
		},
		{
			name: "TypeScript interfaces and types",
			ext:  "ts",
			content: `interface MyInterface {}
type MyType = string | number
function myFunc(x: MyType) {}
`,
			expected: []string{"MyInterface", "MyType", "myFunc"},
		},
		{
			name: "Rust fns and structs",
			ext:  "rs",
			content: `fn my_func() {}
struct MyStruct {}
enum MyEnum {}
trait MyTrait {}
mod my_mod {}
type MyType = u32;
`,
			expected: []string{"my_func", "MyStruct", "MyEnum", "MyTrait", "my_mod", "MyType"},
		},
		{
			name: "C functions and defines",
			ext:  "c",
			content: `#define MAX_VAL 100
struct MyStruct {};
enum MyEnum {};
int main() { return 0; }
void helper_func(int x) { }
`,
			expected: []string{"MAX_VAL", "MyStruct", "MyEnum", "main", "helper_func"},
		},
		{
			name: "C++ classes",
			ext:  "cpp",
			content: `class MyClass {};
struct MyStruct {};
int MyFunc() { return 0; }
`,
			expected: []string{"MyClass", "MyStruct", "MyFunc"},
		},
		{
			name:     "Unsupported extension",
			ext:      "txt",
			content:  "some text",
			expected: nil,
		},
		{
			name:     "Empty content",
			ext:      "go",
			content:  "",
			expected: nil,
		},
		{
			name: "No matches",
			ext:  "go",
			content: `package main
// Just comments
// No symbols here
`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSymbols(tt.ext, tt.content)
			sort.Strings(got)
			sort.Strings(tt.expected)

			if len(got) == 0 && len(tt.expected) == 0 {
				return
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ExtractSymbols() = %v, want %v", got, tt.expected)
			}
		})
	}
}
