package compile

import (
    "go/ast"
    "go/token"
    "github.com/nber1994/akiDsl/runCxt"
    "github.com/nber1994/akiDsl/dslCxt"
)

type CompileCxt struct {
    RunCxt *runCxt.RunCxt //运行时内存空间
    FAst *ast.File
    Fset *token.FileSet
    DslCxt *dslCxt.DslCxt
    ReturnCh chan interface{}
    ErrCh chan error
}

func New(fAst *ast.File, fset *token.FileSet, dslCxtNode *dslCxt.DslCxt) *CompileCxt {
    rct := runCxt.NewRunCxt()
    retChan := make(chan interface{})
    errChan := make(chan error)
    return &CompileCxt {
        RunCxt: rct,
        FAst: fAst,
        Fset: fset,
        DslCxt: dslCxtNode,
        ReturnCh: retChan,
        ErrCh: errChan,
    }
}

//定义一个空方法，不然傻逼gc会在方法结束以后，把结构体回收
func (this *CompileCxt) Rescue() {
}
