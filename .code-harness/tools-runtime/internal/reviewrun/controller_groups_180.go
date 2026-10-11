package reviewrun

import (
    "fmt"
    "path/filepath"
    "sort"
    "strings"
)

// groupControllerChains180 changes only the user-facing choice dimension.
// buildBoundedChain180 and CHANGES impact attribution remain at method/SQL
// statement granularity. A Controller option is an immutable aggregate of
// those exact method chains, never an instruction to review every method in a
// class.
func groupControllerChains180(methods []Chain, mode string) []Chain {
    type bucket struct {
        key, module, path, class string
        methods []Chain
    }
    byKey := make(map[string]*bucket)
    for _, method := range methods {
        if len(method.Nodes) == 0 {
            // An endpoint without even a Controller location has no identity;
            // retain a distinct option rather than silently merging it.
            continue
        }
        node := method.Nodes[0]
        path := filepath.ToSlash(filepath.Clean(node.Path))
        symbol := node.Symbol
        class := symbol
        if at := strings.Index(class, "."); at > 0 {
            class = class[:at]
        }
        module, ok := mavenModuleRoot180(path)
        if !ok {
            module = path // conservative fallback preserves identity
        }
        key := module + "\x00" + path + "\x00" + class
        group := byKey[key]
        if group == nil {
            group = &bucket{key:key, module:module, path:path, class:class}
            byKey[key] = group
        }
        group.methods = append(group.methods,method)
    }
    groups := make([]*bucket,0,len(byKey))
    for _,b := range byKey {
        sort.Slice(b.methods,func(i,j int)bool {
            if b.methods[i].Name!=b.methods[j].Name {return b.methods[i].Name<b.methods[j].Name}
            return b.methods[i].Nodes[0].Symbol<b.methods[j].Nodes[0].Symbol
        })
        groups=append(groups,b)
    }
    sort.Slice(groups,func(i,j int)bool {
        if groups[i].class!=groups[j].class {return groups[i].class<groups[j].class}
        return groups[i].key<groups[j].key
    })
    names:=map[string]int{}
    for _,b:=range groups { names[b.class]++ }
    out:=make([]Chain,0,len(groups))
    for i,b:=range groups {
        choiceID:=fmt.Sprintf("C%d",i+1)
        label:=b.class
        if names[b.class]>1 {
            label+= " ["+b.path+"]"
        }
        suffix:="个受影响方法"
        if mode=="CURRENT_IMPLEMENTATION" {suffix="个方法"}
        label+=fmt.Sprintf("（%d %s）",len(b.methods),suffix)
        group:=Chain{
            ID:choiceID, Name:label, Module:b.module,
            ControllerPath:b.path, Nodes:[]Node{}, Unresolved:[]string{},
            Methods:make([]Chain,0,len(b.methods)),
        }
        seen:=map[string]bool{}
        for j,m:=range b.methods {
            m.ID=fmt.Sprintf("%s.M%d",choiceID,j+1)
            group.Methods=append(group.Methods,m)
            group.Unresolved=append(group.Unresolved,m.Unresolved...)
            for _,node:=range m.Nodes {
                k:=node.Path+"\x00"+node.Symbol+"\x00"+node.Role+"\x00"+node.Workspace
                if !seen[k] {seen[k]=true;group.Nodes=append(group.Nodes,node)}
            }
        }
        group.Unresolved=uniqueStrings180(group.Unresolved)
        out=append(out,group)
    }
    return out
}

// Scope and source-evidence validation operate only on the method-level
// subchains chosen by the human Controller-level selection. Legacy isolated
// T1/T3 tests without Methods keep their existing semantics.
func expandSelectedMethodChains180(choices []Chain) []Chain {
    result:=[]Chain{}
    for _,choice:=range choices {
        if len(choice.Methods)>0 {
            result=append(result,choice.Methods...)
        } else {
            result=append(result,choice)
        }
    }
    return result
}
