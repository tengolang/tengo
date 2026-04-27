// Package format implements canonical formatting of Tengo source code.
package format

import (
	"bytes"
	"strings"

	"github.com/tengolang/tengo/v3/parser"
	"github.com/tengolang/tengo/v3/token"
)

// Format parses src as Tengo source, canonically formats it, and returns the
// result. Returns a parse error unchanged if src is not valid Tengo.
func Format(src []byte) ([]byte, error) {
	fset := parser.NewFileSet()
	sf := fset.AddFile("", -1, len(src))
	p := parser.NewParser(sf, src, nil)
	parsed, err := p.ParseFile()
	if err != nil {
		return nil, err
	}
	cmts := collectComments(sf, src)
	pr := &printer{sf: sf, cmts: cmts}
	pr.printFile(parsed)
	b := pr.buf.Bytes()
	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	return b, nil
}

// comment holds a scanned comment with its source line number.
type comment struct {
	line int
	text string
}

func collectComments(sf *parser.SourceFile, src []byte) []*comment {
	var list []*comment
	// Second pass with ScanComments; AddLine calls are no-ops since the
	// first parse already registered all line offsets in sf.
	s := parser.NewScanner(sf, src, nil, parser.ScanComments)
	for {
		tok, lit, pos := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.Comment {
			list = append(list, &comment{
				line: sf.Position(pos).Line,
				text: lit,
			})
		}
	}
	return list
}

type printer struct {
	buf   bytes.Buffer
	sf    *parser.SourceFile
	cmts  []*comment
	ci    int // index of next unprocessed comment
	depth int // current indentation depth
}

func (p *printer) srcLine(pos parser.Pos) int {
	if !pos.IsValid() {
		return 0
	}
	return p.sf.Position(pos).Line
}

func (p *printer) endLine(n parser.Node) int {
	end := int(n.End()) - 1
	if end <= 0 {
		return p.srcLine(n.Pos())
	}
	return p.sf.Position(parser.Pos(end)).Line
}

func (p *printer) tab() string {
	return strings.Repeat("\t", p.depth)
}

// flushBefore emits all pending comments whose line is < line.
func (p *printer) flushBefore(line int) {
	for p.ci < len(p.cmts) && p.cmts[p.ci].line < line {
		p.buf.WriteString(p.tab())
		p.buf.WriteString(p.cmts[p.ci].text)
		p.buf.WriteByte('\n')
		p.ci++
	}
}

// trailing emits a comment on line as a trailing inline comment (if any).
func (p *printer) trailing(line int) {
	if p.ci < len(p.cmts) && p.cmts[p.ci].line == line {
		p.buf.WriteByte(' ')
		p.buf.WriteString(p.cmts[p.ci].text)
		p.ci++
	}
}

func (p *printer) printFile(f *parser.File) {
	p.stmtList(f.Stmts)
	for p.ci < len(p.cmts) {
		p.buf.WriteString(p.cmts[p.ci].text)
		p.buf.WriteByte('\n')
		p.ci++
	}
}

func (p *printer) stmtList(stmts []parser.Stmt) {
	prevEnd := 0
	for i, s := range stmts {
		if _, ok := s.(*parser.EmptyStmt); ok {
			continue
		}
		stmtLine := p.srcLine(s.Pos())

		if i > 0 && prevEnd > 0 && stmtLine-prevEnd > 1 {
			p.buf.WriteByte('\n') // preserve one blank line
		}

		p.flushBefore(stmtLine)
		p.buf.WriteString(p.tab())
		p.stmt(s)
		end := p.endLine(s)
		p.trailing(end)
		p.buf.WriteByte('\n')

		if prevEnd = end; prevEnd == 0 {
			prevEnd = stmtLine
		}
	}
}

func (p *printer) stmt(s parser.Stmt) {
	switch s := s.(type) {
	case *parser.AssignStmt:
		p.assignStmt(s)
	case *parser.ExprStmt:
		p.expr(s.Expr, token.LowestPrec)
	case *parser.IncDecStmt:
		p.expr(s.Expr, token.LowestPrec)
		p.buf.WriteString(s.Token.String())
	case *parser.BranchStmt:
		p.buf.WriteString(s.Token.String())
		if s.Label != nil {
			p.buf.WriteByte(' ')
			p.buf.WriteString(s.Label.Name)
		}
	case *parser.ReturnStmt:
		p.returnStmt(s)
	case *parser.ExportStmt:
		p.buf.WriteString("export ")
		p.expr(s.Result, token.LowestPrec)
	case *parser.IfStmt:
		p.ifStmt(s)
	case *parser.ForStmt:
		p.forStmt(s)
	case *parser.ForInStmt:
		p.forInStmt(s)
	case *parser.SwitchStmt:
		p.switchStmt(s)
	case *parser.BlockStmt:
		p.block(s)
	case *parser.EmptyStmt:
		// skip
	case *parser.BadStmt:
		p.buf.WriteString("<bad statement>")
	}
}

func (p *printer) assignStmt(s *parser.AssignStmt) {
	for i, e := range s.LHS {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		p.expr(e, token.LowestPrec)
	}
	p.buf.WriteByte(' ')
	p.buf.WriteString(s.Token.String())
	p.buf.WriteByte(' ')
	for i, e := range s.RHS {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		p.expr(e, token.LowestPrec)
	}
}

func (p *printer) returnStmt(s *parser.ReturnStmt) {
	p.buf.WriteString("return")
	for i, r := range s.Results {
		if i == 0 {
			p.buf.WriteByte(' ')
		} else {
			p.buf.WriteString(", ")
		}
		p.expr(r, token.LowestPrec)
	}
}

func (p *printer) ifStmt(s *parser.IfStmt) {
	p.buf.WriteString("if ")
	if s.Init != nil {
		p.stmt(s.Init)
		p.buf.WriteString("; ")
	}
	p.expr(s.Cond, token.LowestPrec)
	p.buf.WriteByte(' ')
	p.block(s.Body)
	if s.Else != nil {
		p.buf.WriteString(" else ")
		switch e := s.Else.(type) {
		case *parser.IfStmt:
			p.ifStmt(e)
		case *parser.BlockStmt:
			p.block(e)
		}
	}
}

func (p *printer) forStmt(s *parser.ForStmt) {
	p.buf.WriteString("for ")
	if s.Init != nil || s.Post != nil {
		if s.Init != nil {
			p.stmt(s.Init)
		}
		p.buf.WriteString("; ")
		if s.Cond != nil {
			p.expr(s.Cond, token.LowestPrec)
		}
		p.buf.WriteString("; ")
		if s.Post != nil {
			p.stmt(s.Post)
		}
		p.buf.WriteByte(' ')
	} else if s.Cond != nil {
		p.expr(s.Cond, token.LowestPrec)
		p.buf.WriteByte(' ')
	}
	p.block(s.Body)
}

func (p *printer) forInStmt(s *parser.ForInStmt) {
	p.buf.WriteString("for ")
	p.buf.WriteString(s.Key.Name)
	if s.Value != nil {
		p.buf.WriteString(", ")
		p.buf.WriteString(s.Value.Name)
	}
	p.buf.WriteString(" in ")
	p.expr(s.Iterable, token.LowestPrec)
	p.buf.WriteByte(' ')
	p.block(s.Body)
}

func (p *printer) switchStmt(s *parser.SwitchStmt) {
	p.buf.WriteString("switch ")
	if s.Init != nil {
		p.stmt(s.Init)
		p.buf.WriteString("; ")
	}
	if s.Tag != nil {
		p.expr(s.Tag, token.LowestPrec)
		p.buf.WriteByte(' ')
	}
	p.buf.WriteString("{\n")
	for _, cs := range s.Body.Stmts {
		cc, ok := cs.(*parser.CaseClause)
		if !ok {
			continue
		}
		ccLine := p.srcLine(cc.Pos())
		p.flushBefore(ccLine)
		p.buf.WriteString(p.tab()) // case at same depth as switch
		if cc.List == nil {
			p.buf.WriteString("default:")
		} else {
			p.buf.WriteString("case ")
			for i, e := range cc.List {
				if i > 0 {
					p.buf.WriteString(", ")
				}
				p.expr(e, token.LowestPrec)
			}
			p.buf.WriteByte(':')
		}
		p.trailing(ccLine)
		p.buf.WriteByte('\n')
		p.depth++ // body indented one level deeper
		p.stmtList(cc.Body)
		p.depth--
	}
	p.buf.WriteString(p.tab())
	p.buf.WriteByte('}')
}

func (p *printer) block(b *parser.BlockStmt) {
	p.buf.WriteString("{\n")
	p.depth++
	p.stmtList(b.Stmts)
	closeLine := p.srcLine(b.RBrace)
	p.flushBefore(closeLine)
	p.depth--
	p.buf.WriteString(p.tab())
	p.buf.WriteByte('}')
}

func (p *printer) expr(e parser.Expr, minPrec int) {
	switch e := e.(type) {
	case *parser.BinaryExpr:
		prec := e.Token.Precedence()
		if prec < minPrec {
			p.buf.WriteByte('(')
			p.binaryExpr(e)
			p.buf.WriteByte(')')
		} else {
			p.binaryExpr(e)
		}
	case *parser.UnaryExpr:
		p.buf.WriteString(e.Token.String())
		p.expr(e.Expr, token.LowestPrec)
	case *parser.ParenExpr:
		p.buf.WriteByte('(')
		p.expr(e.Expr, token.LowestPrec)
		p.buf.WriteByte(')')
	case *parser.Ident:
		p.buf.WriteString(e.Name)
	case *parser.IntLit:
		p.buf.WriteString(e.Literal)
	case *parser.FloatLit:
		p.buf.WriteString(e.Literal)
	case *parser.StringLit:
		p.buf.WriteString(e.Literal)
	case *parser.CharLit:
		p.buf.WriteString(e.Literal)
	case *parser.BoolLit:
		p.buf.WriteString(e.Literal)
	case *parser.UndefinedLit:
		p.buf.WriteString("undefined")
	case *parser.ArrayLit:
		p.arrayLit(e)
	case *parser.MapLit:
		p.mapLit(e)
	case *parser.FuncLit:
		p.funcLit(e)
	case *parser.CallExpr:
		p.callExpr(e)
	case *parser.MethodCallExpr:
		p.methodCallExpr(e)
	case *parser.SelectorExpr:
		p.expr(e.Expr, token.LowestPrec)
		p.buf.WriteByte('.')
		p.expr(e.Sel, token.LowestPrec)
	case *parser.IndexExpr:
		p.expr(e.Expr, token.LowestPrec)
		p.buf.WriteByte('[')
		if e.Index != nil {
			p.expr(e.Index, token.LowestPrec)
		}
		p.buf.WriteByte(']')
	case *parser.SliceExpr:
		p.expr(e.Expr, token.LowestPrec)
		p.buf.WriteByte('[')
		if e.Low != nil {
			p.expr(e.Low, token.LowestPrec)
		}
		p.buf.WriteByte(':')
		if e.High != nil {
			p.expr(e.High, token.LowestPrec)
		}
		p.buf.WriteByte(']')
	case *parser.CondExpr:
		p.expr(e.Cond, token.LowestPrec)
		p.buf.WriteString(" ? ")
		p.expr(e.True, token.LowestPrec)
		p.buf.WriteString(" : ")
		p.expr(e.False, token.LowestPrec)
	case *parser.ImportExpr:
		p.buf.WriteString(`import("`)
		p.buf.WriteString(e.ModuleName)
		p.buf.WriteString(`")`)
	case *parser.ErrorExpr:
		p.buf.WriteString("error(")
		p.expr(e.Expr, token.LowestPrec)
		p.buf.WriteByte(')')
	case *parser.ImmutableExpr:
		p.buf.WriteString("immutable(")
		p.expr(e.Expr, token.LowestPrec)
		p.buf.WriteByte(')')
	case *parser.BadExpr:
		p.buf.WriteString("<bad expression>")
	}
}

func (p *printer) binaryExpr(e *parser.BinaryExpr) {
	prec := e.Token.Precedence()
	p.expr(e.LHS, prec)
	p.buf.WriteByte(' ')
	p.buf.WriteString(e.Token.String())
	p.buf.WriteByte(' ')
	p.expr(e.RHS, prec)
}

func (p *printer) arrayLit(e *parser.ArrayLit) {
	p.buf.WriteByte('[')
	for i, el := range e.Elements {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		p.expr(el, token.LowestPrec)
	}
	p.buf.WriteByte(']')
}

func (p *printer) mapLit(e *parser.MapLit) {
	if len(e.Elements) == 0 {
		p.buf.WriteString("{}")
		return
	}
	p.buf.WriteString("{\n")
	p.depth++
	for _, el := range e.Elements {
		p.buf.WriteString(p.tab())
		p.buf.WriteString(el.Key)
		p.buf.WriteString(": ")
		p.expr(el.Value, token.LowestPrec)
		p.buf.WriteString(",\n")
	}
	p.depth--
	p.buf.WriteString(p.tab())
	p.buf.WriteByte('}')
}

func (p *printer) funcLit(e *parser.FuncLit) {
	p.buf.WriteString("func(")
	params := e.Type.Params
	for i, param := range params.List {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		if params.VarArgs && i == len(params.List)-1 {
			p.buf.WriteString("...")
		}
		p.buf.WriteString(param.Name)
	}
	p.buf.WriteString(") ")
	p.block(e.Body)
}

func (p *printer) callExpr(e *parser.CallExpr) {
	p.expr(e.Func, token.LowestPrec)
	p.buf.WriteByte('(')
	for i, arg := range e.Args {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		p.expr(arg, token.LowestPrec)
	}
	if e.Ellipsis.IsValid() && len(e.Args) > 0 {
		p.buf.WriteString("...")
	}
	p.buf.WriteByte(')')
}

func (p *printer) methodCallExpr(e *parser.MethodCallExpr) {
	p.expr(e.Recv, token.LowestPrec)
	p.buf.WriteString("::")
	p.buf.WriteString(e.Method.Value)
	p.buf.WriteByte('(')
	for i, arg := range e.Args {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		p.expr(arg, token.LowestPrec)
	}
	if e.Ellipsis.IsValid() && len(e.Args) > 0 {
		p.buf.WriteString("...")
	}
	p.buf.WriteByte(')')
}
