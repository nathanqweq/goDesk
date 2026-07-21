package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTestWritableSucceedsAndLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "godesk-config.yaml")

	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatalf("erro ao preparar arquivo: %v", err)
	}

	if err := TestWritable(path); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("erro ao listar dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "godesk-config.yaml" {
		t.Fatalf("esperava só o arquivo original no dir, achei: %v", entries)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("erro ao reler arquivo: %v", err)
	}
	if string(got) != "original" {
		t.Fatalf("TestWritable não deveria alterar o conteúdo do arquivo real, veio %q", got)
	}
}

func TestTestWritableFailsWhenDirDoesNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir-inexistente", "godesk-config.yaml")

	if err := TestWritable(path); err == nil {
		t.Fatal("esperava erro para diretório inexistente")
	}
}

func TestSaveFileCreatesBackupOfPreviousContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "godesk-config.yaml")

	if err := os.WriteFile(path, []byte("versao antiga"), 0644); err != nil {
		t.Fatalf("erro ao preparar arquivo: %v", err)
	}

	backup, err := SaveFile(path, []byte("versao nova"))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if backup == "" {
		t.Fatal("esperava caminho de backup não vazio (arquivo já existia)")
	}

	gotBackup, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("erro ao ler backup: %v", err)
	}
	if string(gotBackup) != "versao antiga" {
		t.Fatalf("backup deveria ter o conteúdo antigo, veio %q", gotBackup)
	}

	gotNew, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("erro ao reler arquivo: %v", err)
	}
	if string(gotNew) != "versao nova" {
		t.Fatalf("arquivo deveria ter o conteúdo novo, veio %q", gotNew)
	}
}

func TestSaveFileNoBackupWhenFileDidNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "godesk-config.yaml")

	backup, err := SaveFile(path, []byte("conteudo"))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if backup != "" {
		t.Fatalf("esperava backup vazio (arquivo não existia antes), veio %q", backup)
	}
}
