package goanalysis

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func writeTempGo(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScanFile_ParseFilesAndExecuteWithStruct(t *testing.T) {
	src := `package main

import (
	"html/template"
	"net/http"
)

type HomeData struct {
	Username string
	Age      int
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("index.gohtml"))
	tpl.Execute(w, HomeData{Username: "x", Age: 1})
}
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].TemplateName == "index.gohtml" && bindings[i].DataType != nil {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with TemplateName 'index.gohtml' and non-nil DataType")
	}
	if found.DataType.Name != "HomeData" {
		t.Fatalf("expected DataType.Name 'HomeData', got %q", found.DataType.Name)
	}
	usernameField, ok := found.DataType.Fields["Username"]
	if !ok {
		t.Fatal("expected field 'Username'")
	}
	if usernameField.TypeName != "string" {
		t.Fatalf("expected Username type 'string', got %q", usernameField.TypeName)
	}
	ageField, ok := found.DataType.Fields["Age"]
	if !ok {
		t.Fatal("expected field 'Age'")
	}
	if ageField.TypeName != "int" {
		t.Fatalf("expected Age type 'int', got %q", ageField.TypeName)
	}
}

func TestScanFile_ExecuteTemplate(t *testing.T) {
	src := `package main

import (
	"html/template"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("layout.gohtml"))
	data := map[string]string{}
	tpl.ExecuteTemplate(w, "header", data)
}
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	var found bool
	for _, b := range bindings {
		if b.TemplateName == "header" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected binding with TemplateName 'header'")
	}
}

func TestScanFile_FuncMap(t *testing.T) {
	src := `package main

import (
	"html/template"
	"strings"
)

var tpl = template.Must(
	template.New("main").Funcs(template.FuncMap{"upper": strings.ToUpper}).ParseFiles("main.gohtml"),
)
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	var found *TemplateBinding
	for i := range bindings {
		if _, ok := bindings[i].FuncMaps["upper"]; ok {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with FuncMaps containing 'upper'")
	}
}

func TestScanFile_PointerData(t *testing.T) {
	src := `package main

import (
	"html/template"
	"net/http"
)

type PageData struct {
	Title string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("page.gohtml"))
	tpl.Execute(w, &PageData{Title: "Hi"})
}
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "PageData" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'PageData'")
	}
	if _, ok := found.DataType.Fields["Title"]; !ok {
		t.Fatal("expected field 'Title' in PageData")
	}
}

func TestScanFile_NoTemplateUsage(t *testing.T) {
	src := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)
	if len(bindings) != 0 {
		t.Fatalf("expected 0 bindings, got %d", len(bindings))
	}
}

func TestBuiltinFuncs(t *testing.T) {
	funcs := BuiltinFuncs()

	expected := []string{"len", "eq", "printf", "and", "or", "not", "gt", "lt"}
	for _, name := range expected {
		sig, ok := funcs[name]
		if !ok {
			t.Errorf("expected builtin func %q", name)
			continue
		}
		if sig.Name == "" {
			t.Errorf("builtin %q has empty Name", name)
		}
		if sig.Signature == "" {
			t.Errorf("builtin %q has empty Signature", name)
		}
	}
}

func TestScanFile_VariableDataArg(t *testing.T) {
	src := `package main

import (
	"html/template"
	"net/http"
)

type UserData struct {
	Name  string
	Email string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("user.gohtml"))
	data := UserData{Name: "Alice", Email: "alice@example.com"}
	tpl.Execute(w, data)
}
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "UserData" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'UserData' resolved from variable")
	}
	if _, ok := found.DataType.Fields["Name"]; !ok {
		t.Fatal("expected field 'Name'")
	}
	if _, ok := found.DataType.Fields["Email"]; !ok {
		t.Fatal("expected field 'Email'")
	}
}

func TestScanFile_VariablePointerDataArg(t *testing.T) {
	src := `package main

import (
	"html/template"
	"net/http"
)

type Config struct {
	Host string
	Port int
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("config.gohtml"))
	cfg := &Config{Host: "localhost", Port: 8080}
	tpl.Execute(w, cfg)
}
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "Config" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'Config' resolved from pointer variable")
	}
	if _, ok := found.DataType.Fields["Host"]; !ok {
		t.Fatal("expected field 'Host'")
	}
}

func TestScanFile_VarDeclDataArg(t *testing.T) {
	src := `package main

import (
	"html/template"
	"net/http"
)

type Settings struct {
	Debug bool
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("settings.gohtml"))
	var s Settings
	tpl.Execute(w, s)
}
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "Settings" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'Settings' resolved from var declaration")
	}
}

func TestScanFile_MustParseFiles(t *testing.T) {
	src := `package main

import "html/template"

var templates = template.Must(template.ParseFiles(
	"templates/base.gohtml",
	"templates/header.gohtml",
	"templates/footer.gohtml",
))
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	if len(bindings) == 0 {
		t.Fatal("expected at least one binding")
	}
	b := bindings[0]
	if len(b.ParsedFiles) != 3 {
		t.Fatalf("expected 3 parsed files, got %d", len(b.ParsedFiles))
	}
}

func TestScanFile_HtmlTemplatePackage(t *testing.T) {
	// Ensure html/template is handled the same as text/template
	src := `package main

import (
	"html/template"
	"net/http"
)

type Page struct {
	Title string
	Body  string
}

func handler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles("page.gohtml"))
	t.Execute(w, Page{Title: "Hello", Body: "World"})
}
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "Page" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'Page' from html/template")
	}
	if _, ok := found.DataType.Fields["Title"]; !ok {
		t.Fatal("expected field 'Title'")
	}
	if _, ok := found.DataType.Fields["Body"]; !ok {
		t.Fatal("expected field 'Body'")
	}
}

func TestScanFile_MethodChain(t *testing.T) {
	src := `package main

import (
	"html/template"
	"strings"
	"net/http"
)

type Item struct {
	Name  string
	Price float64
}

func handler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(
		template.New("shop").Funcs(template.FuncMap{
			"upper": strings.ToUpper,
			"lower": strings.ToLower,
		}).ParseFiles("shop.gohtml"),
	)
	t.Execute(w, Item{Name: "Widget", Price: 9.99})
}
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	// Check funcmap
	var hasUpper, hasLower bool
	for _, b := range bindings {
		if _, ok := b.FuncMaps["upper"]; ok {
			hasUpper = true
		}
		if _, ok := b.FuncMaps["lower"]; ok {
			hasLower = true
		}
	}
	if !hasUpper {
		t.Fatal("expected FuncMap to contain 'upper'")
	}
	if !hasLower {
		t.Fatal("expected FuncMap to contain 'lower'")
	}

	// Check data type
	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "Item" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'Item'")
	}
}

func TestScanFile_ParseGlob(t *testing.T) {
	src := `package main

import "html/template"

var templates = template.Must(template.ParseGlob("templates/*.gohtml"))
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	if len(bindings) == 0 {
		t.Fatal("expected at least one binding from ParseGlob")
	}
	found := false
	for _, b := range bindings {
		for _, pf := range b.ParsedFiles {
			if pf == "templates/*.gohtml" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected parsed files to contain glob pattern")
	}
}

func TestScanFile_EmbeddedStruct(t *testing.T) {
	src := `package main

import (
	"html/template"
	"net/http"
)

type Base struct {
	Title string
}

type PageData struct {
	Base
	Content string
}

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("page.gohtml"))
	tpl.Execute(w, PageData{Content: "hello"})
}
`
	path := writeTempGo(t, src)
	bindings := ScanFile(path)

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "PageData" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'PageData'")
	}
	if _, ok := found.DataType.Fields["Content"]; !ok {
		t.Fatal("expected field 'Content'")
	}
	if _, ok := found.DataType.Fields["Base"]; !ok {
		t.Fatal("expected embedded field 'Base'")
	}
}

func TestScanDirectory_CrossFileStruct(t *testing.T) {
	dir := t.TempDir()

	// types.go defines the struct
	types := `package main

type AppData struct {
	AppName string
	Version string
}
`
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(types), 0644); err != nil {
		t.Fatal(err)
	}

	// main.go uses it
	main := `package main

import (
	"html/template"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("index.gohtml"))
	tpl.Execute(w, AppData{AppName: "MyApp", Version: "1.0"})
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0644); err != nil {
		t.Fatal(err)
	}

	bindings := ScanDirectory(dir)

	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "AppData" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'AppData' resolved across files")
	}
	if _, ok := found.DataType.Fields["AppName"]; !ok {
		t.Fatal("expected field 'AppName'")
	}
	if _, ok := found.DataType.Fields["Version"]; !ok {
		t.Fatal("expected field 'Version'")
	}
}

func TestResolveTypeInfo_NestedStructs(t *testing.T) {
	structs := map[string][]FieldInfo{
		"Page": {
			{Name: "Title", TypeName: "string"},
			{Name: "Author", TypeName: "User"},
		},
		"User": {
			{Name: "Name", TypeName: "string"},
			{Name: "Email", TypeName: "string"},
			{Name: "Address", TypeName: "Address"},
		},
		"Address": {
			{Name: "City", TypeName: "string"},
			{Name: "Country", TypeName: "string"},
		},
	}
	methods := map[string][]MethodInfo{
		"User": {{Name: "FullName", Signature: "func FullName() string", Returns: "string"}},
	}

	ti := ResolveTypeInfo("Page", structs, methods)
	if ti.Name != "Page" {
		t.Fatalf("expected Page, got %s", ti.Name)
	}
	authorField, ok := ti.Fields["Author"]
	if !ok {
		t.Fatal("expected Author field")
	}
	if authorField.ChildType == nil {
		t.Fatal("expected Author to have ChildType")
	}
	if authorField.ChildType.Name != "User" {
		t.Fatalf("expected User, got %s", authorField.ChildType.Name)
	}
	if _, ok := authorField.ChildType.Fields["Name"]; !ok {
		t.Fatal("expected User.Name field")
	}
	if _, ok := authorField.ChildType.Methods["FullName"]; !ok {
		t.Fatal("expected User.FullName method")
	}
	// Nested: User.Address.City
	addrField, ok := authorField.ChildType.Fields["Address"]
	if !ok {
		t.Fatal("expected User.Address field")
	}
	if addrField.ChildType == nil {
		t.Fatal("expected Address ChildType")
	}
	if _, ok := addrField.ChildType.Fields["City"]; !ok {
		t.Fatal("expected Address.City field")
	}
}

func TestResolveFieldChain(t *testing.T) {
	structs := map[string][]FieldInfo{
		"Page": {{Name: "Author", TypeName: "User"}},
		"User": {{Name: "Name", TypeName: "string"}},
	}
	ti := ResolveTypeInfo("Page", structs, nil)

	result := ResolveFieldChain(ti, []string{"Author"})
	if result == nil {
		t.Fatal("expected resolved User type")
	}
	if result.Name != "User" {
		t.Fatalf("expected User, got %s", result.Name)
	}

	// Non-existent chain
	result = ResolveFieldChain(ti, []string{"Missing"})
	if result != nil {
		t.Fatal("expected nil for missing field")
	}
}

func TestCollectMethods_FromSource(t *testing.T) {
	src := `package main

type User struct {
	Name string
}

// FullName returns the user's full name.
func (u User) FullName() string {
	return u.Name
}

func (u *User) SetName(name string) {
	u.Name = name
}
`
	path := writeTempGo(t, src)
	srcBytes, _ := os.ReadFile(path)
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, path, srcBytes, parser.ParseComments)
	methods := CollectMethods(file, fset)

	userMethods := methods["User"]
	if len(userMethods) < 2 {
		t.Fatalf("expected at least 2 methods on User, got %d", len(userMethods))
	}
	var hasFullName, hasSetName bool
	for _, m := range userMethods {
		if m.Name == "FullName" {
			hasFullName = true
			if m.Returns != "string" {
				t.Errorf("expected FullName returns 'string', got %q", m.Returns)
			}
		}
		if m.Name == "SetName" {
			hasSetName = true
		}
	}
	if !hasFullName {
		t.Error("expected FullName method")
	}
	if !hasSetName {
		t.Error("expected SetName method")
	}
}

func TestScanDirectory_MethodsResolved(t *testing.T) {
	dir := t.TempDir()

	types := `package main

type User struct {
	Name string
}

func (u User) Greeting() string {
	return "Hello " + u.Name
}
`
	main := `package main

import (
	"html/template"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFiles("user.gohtml"))
	tpl.Execute(w, User{Name: "Alice"})
}
`
	os.WriteFile(filepath.Join(dir, "types.go"), []byte(types), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(main), 0644)

	bindings := ScanDirectory(dir)
	var found *TemplateBinding
	for i := range bindings {
		if bindings[i].DataType != nil && bindings[i].DataType.Name == "User" {
			found = &bindings[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected binding with DataType 'User'")
	}
	if _, ok := found.DataType.Methods["Greeting"]; !ok {
		t.Fatal("expected method 'Greeting' on User")
	}
}

func TestNewTypeInfo(t *testing.T) {
	ti := NewTypeInfo("Foo", "main")
	if ti.Name != "Foo" {
		t.Fatalf("expected Name 'Foo', got %q", ti.Name)
	}
	if ti.Package != "main" {
		t.Fatalf("expected Package 'main', got %q", ti.Package)
	}
	if ti.Fields == nil {
		t.Fatal("expected non-nil Fields map")
	}
	if ti.Methods == nil {
		t.Fatal("expected non-nil Methods map")
	}
	if len(ti.Fields) != 0 {
		t.Fatalf("expected empty Fields, got %d", len(ti.Fields))
	}
	if len(ti.Methods) != 0 {
		t.Fatalf("expected empty Methods, got %d", len(ti.Methods))
	}
}
