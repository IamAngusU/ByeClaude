package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrePushHookLifecycleAndIsolation(t *testing.T) {
	dir:=t.TempDir()
	runGit(t,dir,"init","-q")
	if err:=runHook([]string{"pre-push-install","--repo",dir});err!=nil{t.Fatal(err)}
	path:=filepath.Join(dir,".git","hooks","pre-push")
	content,err:=os.ReadFile(path)
	if err!=nil{t.Fatal(err)}
	if !strings.Contains(string(content),"pre-push-filter") {t.Fatalf("incorrect hook: %q",content)}
	if _,err:=os.Stat(filepath.Join(dir,".git","hooks","commit-msg"));!os.IsNotExist(err) {
		t.Fatal("pre-push-install must not implicitly mutate commit-msg")
	}
	if err:=runHook([]string{"pre-push-remove","--repo",dir});err!=nil{t.Fatal(err)}
	if _,err:=os.Stat(path);!os.IsNotExist(err){t.Fatalf("pre-push hook remains: %v",err)}
}

func TestPrePushHookRefusesUnrelatedExistingHook(t *testing.T) {
	dir:=t.TempDir()
	runGit(t,dir,"init","-q")
	hook:=filepath.Join(dir,".git","hooks","pre-push")
	const existing="#!/bin/sh\nexit 0\n"
	if err:=os.WriteFile(hook,[]byte(existing),0755);err!=nil{t.Fatal(err)}
	if err:=runHook([]string{"pre-push-install","--repo",dir});err==nil {
		t.Fatal("must refuse overwriting an existing pre-push hook")
	}
	out,err:=os.ReadFile(hook)
	if err!=nil||string(out)!=existing{t.Fatalf("hook changed unexpectedly: %s, %v",out,err)}
}

func TestIdentityCLIPlanFlagsAccepted(t *testing.T) {
	dir:=t.TempDir()
	runGit(t,dir,"init","-q")
	runGit(t,dir,"config","user.name","Human")
	runGit(t,dir,"config","user.email","human@example.org")
	runGit(t,dir,"commit","--allow-empty","-m","Initial")
	if err:=runPlan([]string{"--repo",dir,"--replace-author","Actual Human <human@example.org>"});err!=nil {
		t.Fatalf("read-only plan should accept explicit replacement: %v",err)
	}
	if err:=runClean([]string{"--repo",dir,"--replace-author","Missing email"});err==nil {
		t.Fatal("invalid corrected identity must fail")
	}
}
