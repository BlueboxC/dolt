package merge

import (
 "fmt"
 "github.com/dolthub/dolt/go/libraries/doltcore/doltdb"
 "github.com/dolthub/dolt/go/libraries/doltcore/table/editor"
 "github.com/dolthub/go-mysql-server/sql"
)

// MergeBaseCommit retains the fork's explicit-resolution policy on the upstream ORT resolver.
func MergeBaseCommit(ctx *sql.Context, _ doltdb.TableResolver, left, right *doltdb.Commit, _ editor.Options) (*doltdb.Commit, error) {
 base, _, conflicted, err := resolveMergeBase(ctx,left,right)
 if err != nil { return nil,err }
 if len(conflicted)!=0 { return nil,fmt.Errorf("best common ancestors conflict; virtual merge base requires explicit resolution") }
 return base,nil
}
