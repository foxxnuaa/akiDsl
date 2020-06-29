package compile

import (
    "fmt"
    "go/ast"
    "go/token"
	"github.com/spf13/cast"
    "github.com/nber1994/akiDsl/runCxt"
)

type Stmt struct{
    Rct *runCxt.RunCxt //变量作用空间
    Type int
    Father *Stmt //子节点可以访问到父节点的内存空间
}

func NewStmt() *Stmt {
    rct := runCxt.NewRunCxt()
    return &Stmt{
        Rct: rct,
    }
}

func (this *Stmt) NewChild() *Stmt {
    stmt := NewStmt()
    stmt.Father = this
    return stmt
}

//编译stmt
func (this *Stmt) CompileStmt(cpt *CompileCxt, stmt ast.Stmt) {
    if nil == stmt {
        return
    }
    cStmt := this.NewChild()
    switch stmt := stmt.(type) {
    case *ast.AssignStmt:
        //赋值在本节点的内存中
        this.CompileAssignStmt(cpt, stmt)
    case *ast.IncDecStmt:
        this.CompileIncDecStmt(cpt, stmt)
    case *ast.IfStmt:
        cStmt.CompileIfStmt(cpt, stmt)
    case *ast.ForStmt:
        cStmt.CompileForStmt(cpt, stmt)
    case *ast.RangeStmt:
        cStmt.CompileRangeStmt(cpt, stmt)
    case *ast.ReturnStmt:
        cStmt.CompileReturnStmt(cpt, stmt)
    case *ast.BlockStmt:
        cStmt.CompileBlockStmt(cpt, stmt)
    default:
        panic("syntax error: nonsupport stmt ")
    }
}

func (this *Stmt) CompileBlockStmt(cpt *CompileCxt, stmt *ast.BlockStmt) {
    fmt.Println("-----------------in block stmt")
    for _, b := range stmt.List {
        this.CompileStmt(cpt, b)
    }
}

//获取值 回溯所有父节点获取值
func (this *Stmt) GetValue(name string) interface{} {
    stmt := this
    for nil != stmt {
        fmt.Println("now stmt rct is ", stmt.Rct.ToString())
        if _, exist := stmt.Rct.Vars[name]; exist {
            return stmt.Rct.Vars[name]
        }
        stmt = stmt.Father
    }
    panic("syntax error: non-reachable var " + name)
}

func (this *Stmt) ValueExist(name string) bool {
    ret := false
    stmt := this
    for nil != stmt {
        fmt.Println("now stmt rct is ", stmt.Rct.ToString())
        if _, exist := stmt.Rct.Vars[name]; exist {
            ret = true
        }
        stmt = stmt.Father
    }
    return ret
}

func (this *Stmt) SetValue(name string, value interface{}, create bool) {
    if create {
        //只在本节点内存中做校验
        if this.Rct.ValueExist(name) {
            panic("syntax error: redeclare var " + name)
        }
    } else {
        //只在本节点内存中做校验
        if !this.ValueExist(name) {
            panic("syntax error: undeclare var " + name)
        }
    }
    this.Rct.SetValue(name, value)
}

func (this *Stmt) CompileAssignStmt(cpt *CompileCxt, stmt *ast.AssignStmt) {
    fmt.Println("-----------------in assign stmt")
    //只支持= :=
    if token.DEFINE != stmt.Tok && token.ASSIGN != stmt.Tok {
        panic("syntax error: nonsupport Tok ")
    }

    expr := NewExpr()
    //Lhs中的变量进行声明
    if len(stmt.Lhs) == len(stmt.Rhs) {
        for idx, l := range stmt.Lhs {
            switch l := l.(type) {
            case *ast.Ident:
                r := stmt.Rhs[idx]
                this.SetValue(l.Name, expr.CompileExpr(cpt.DslCxt, this, r), token.DEFINE == stmt.Tok)
            case *ast.IndexExpr:
                r := stmt.Rhs[idx]
                target := expr.CompileExpr(cpt.DslCxt, this, l.X)
                idx := expr.CompileExpr(cpt.DslCxt, this, l.Index)
                switch target := target.(type) {
                case map[interface{}]interface{}:
                    target[idx] = expr.CompileExpr(cpt.DslCxt, this, r)
                    this.SetValue(l.X.(*ast.Ident).Name, target, false)
                case []interface{}:
                    switch idx := idx.(type) {
                    case int:
                        target[idx] = expr.CompileExpr(cpt.DslCxt, this, r)
                    default:
                        panic("syntax error: index type error")
                    }
                    this.SetValue(l.X.(*ast.Ident).Name, target, false)
                default:
                    panic("syntax error: assign type error")
                }
            default:
                panic("syntax error: assign type error")
            }
        }
    } else if len(stmt.Lhs) > len(stmt.Rhs) && 1 == len(stmt.Rhs){
        //声明语句不能嵌套，如果Rhs的元素是方法，则执行多返回值编译逻辑
        r := stmt.Rhs[0]
        switch r := r.(type) {
        case *ast.CallExpr:
            funcRet := expr.CompileCallMultiReturnExpr(cpt.DslCxt, this, r)
            if len(funcRet) != len(funcRet) {
                panic("syntax error: func return can not match")
            }
            for k, l := range stmt.Lhs {
                this.SetValue(l.(*ast.Ident).Name, funcRet[k], token.DEFINE == stmt.Tok)
            }
        case *ast.IndexExpr:
            if 2 == len(stmt.Lhs) && 1 == len(stmt.Rhs) {
                //处理v, exist := a[b]的情况
                target := expr.CompileExpr(cpt.DslCxt, this, stmt.Rhs[0].(*ast.IndexExpr).X)
                switch target := target.(type) {
                case map[interface{}]interface{}:
                    idx := expr.CompileExpr(cpt.DslCxt, this, stmt.Rhs[0].(*ast.IndexExpr).Index)
                    kName := stmt.Lhs[0].(*ast.Ident).Obj.Name
                    vName := stmt.Lhs[1].(*ast.Ident).Obj.Name
                    kVar, vExist := target[idx]
                    this.SetValue(kName, kVar, token.DEFINE == stmt.Tok)
                    this.SetValue(vName, vExist, token.DEFINE == stmt.Tok)
                default:
                    panic("syntax error: index exist assign stmt type error")
                }
            }
        default:
            panic("syntax error: assign nums not match")
        }

    }
}


func (this *Stmt) CompileForStmt(cpt *CompileCxt, stmt *ast.ForStmt) {
    fmt.Println("----------------in for stmt")
    stmtHd := this.NewChild()
    expr := NewExpr()
    //初始条件
    this.CompileStmt(cpt, stmt.Init)
    for {
        if access := expr.CompileExpr(cpt.DslCxt, this, stmt.Cond); !cast.ToBool(access) {
            break;
        }
        //执行body
        stmtHd.CompileStmt(cpt, stmt.Body)
        this.CompileStmt(cpt, stmt.Post)
    }
}

func (this *Stmt) CompileIfStmt(cpt *CompileCxt, stmt *ast.IfStmt) {
    fmt.Println("----------------in if stmt")
    stmtHd := this.NewChild()
    expr := NewExpr()
    //赋值操作,在本节点赋值
    this.CompileStmt(cpt, stmt.Init)
    condRet := expr.CompileExpr(cpt.DslCxt, this, stmt.Cond)
    //如果条件成立
    if cast.ToBool(condRet) {
        stmtHd.CompileStmt(cpt, stmt.Body)
    } else {
        stmtHd.CompileStmt(cpt, stmt.Else)
    }
}

//只支持变量
func (this *Stmt) CompileIncDecStmt(cpt *CompileCxt, stmt *ast.IncDecStmt) {
    fmt.Println("----------------in inc dec stmt")
    //只支持 ++ --
    if token.INC != stmt.Tok && token.DEC != stmt.Tok {
        panic("syntax error: nonsupport Tok ")
    }

    varName := stmt.X.(*ast.Ident).Name
    switch stmt.Tok {
    case token.INC:
        //this.SetValue(varName, expr.CompileExpr(cpt.DslCxt, this, stmt.X))
        this.SetValue(varName, BInc(this.GetValue(varName)), false)
    case token.DEC:
        //this.SetValue(varName, expr.CompileExpr(cpt.DslCxt, this, stmt.X))
        this.SetValue(varName, BDec(this.GetValue(varName)), false)
    default:
        panic("syntax error: nonsupport Tok ")
    }
}

func (this *Stmt) CompileRangeStmt(cpt *CompileCxt, stmt *ast.RangeStmt) {
    fmt.Println("----------------in range stmt")
    expr := NewExpr()
    stmtHd := this.NewChild()
    RangeTarget := expr.CompileExpr(cpt.DslCxt, this, stmt.Key.(*ast.Ident).Obj.Decl.(*ast.AssignStmt).Rhs[0].(*ast.UnaryExpr).X)
    kName := stmt.Key.(*ast.Ident).Name
    vName := stmt.Key.(*ast.Ident).Obj.Decl.(*ast.AssignStmt).Lhs[1].(*ast.Ident).Name
    switch rt := RangeTarget.(type) {
    case []interface{}:
        for k, v := range rt {
            //设置kv的值
            this.SetValue(kName, k, true)
            this.SetValue(vName, v, true)
            //执行Body
            stmtHd.CompileStmt(cpt, stmt.Body)
        }
    case map[interface{}]interface{}:
        for k, v := range rt {
            //设置kv的值
            this.SetValue(kName, k, true)
            this.SetValue(vName, v, true)
            //执行Body
            stmtHd.CompileStmt(cpt, stmt.Body)
        }
    default:
        panic("syntax error: nonsupport range type")
    }
}

//支持返回只支持一个
func (this *Stmt) CompileReturnStmt(cpt *CompileCxt, stmt *ast.ReturnStmt) {
    fmt.Println("----------------in return stmt")
    var ret interface{}
    expr := NewExpr()
    e := stmt.Results[0]
    ret = expr.CompileExpr(cpt.DslCxt, this, e)
    fmt.Println("----------------return ", ret)
    cpt.ReturnCh <- ret
}
