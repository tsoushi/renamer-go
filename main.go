package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
	"golang.org/x/xerrors"
)

var (
	FLAG_DIR         = "dir"
	FLAG_FORMAT      = "format"
	FLAG_DRY         = "dry"
	FLAG_INTERACTIVE = "interactive"
)

func main() {
	cmd := &cli.Command{
		Name:   "renamer-go",
		Usage:  "ファイルのリネームを行うCLIツール",
		Flags:  defineFlags(),
		Action: action,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		panic(err)
	}
}

func defineFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    FLAG_DIR,
			Aliases: []string{"d"},
			Usage:   "指定したディレクトリ内のファイルをすべてリネームする",
		},
		&cli.StringFlag{
			Name:    FLAG_FORMAT,
			Aliases: []string{"f"},
			Usage:   "リネーム後のファイル名のフォーマットを指定する",
			Value:   "{year}{month}{day}_{index}.{ext}",
		},
		&cli.BoolFlag{
			Name:  FLAG_DRY,
			Usage: "実際にはリネームを行わず、リネーム内容のみを表示する",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  FLAG_INTERACTIVE,
			Usage: "リネーム前に確認を行う",
			Value: true,
		},
	}
}

type TargetFile struct {
	Dir     string
	OldName string
	NewName string
	ModTime time.Time
}

type FileGroup struct {
	Dir     string
	NewName string
}

type RenameFile struct {
	OldPath string
	NewPath string
}

func action(ctx context.Context, cmd *cli.Command) error {
	dir := cmd.String(FLAG_DIR)
	format := cmd.String(FLAG_FORMAT)
	dryRun := cmd.Bool(FLAG_DRY)
	interactive := cmd.Bool(FLAG_INTERACTIVE)

	items, err := os.ReadDir(dir)
	if err != nil {
		return xerrors.Errorf("ディレクトリの読み込みに失敗: %w", err)
	}

	targetFileMap := make(map[FileGroup][]TargetFile)

	for _, item := range items {
		if item.IsDir() {
			continue
		}

		info, err := item.Info()
		if err != nil {
			return xerrors.Errorf("ファイル情報の取得に失敗: %w", err)
		}

		modTime := info.ModTime()

		fileGroup := FileGroup{
			Dir:     dir,
			NewName: replaceFormatExceptIndexAndExt(format, item.Name(), modTime),
		}
		targetFile := TargetFile{
			Dir:     dir,
			OldName: item.Name(),
			NewName: "",
			ModTime: modTime,
		}

		if _, exists := targetFileMap[fileGroup]; !exists {
			targetFileMap[fileGroup] = []TargetFile{}
		}

		targetFileMap[fileGroup] = append(targetFileMap[fileGroup], targetFile)
	}

	renameFiles := []RenameFile{}

	for fileGroup, targetFiles := range targetFileMap {
		indexWidth := len(strconv.Itoa(len(targetFiles)))

		// インデックスをファイル更新日時の昇順で振るためにソート
		sort.SliceStable(targetFiles, func(i, j int) bool {
			return targetFiles[i].ModTime.Before(targetFiles[j].ModTime)
		})

		for index, targetFile := range targetFiles {
			newName := strings.ReplaceAll(fileGroup.NewName, "{index}", fmt.Sprintf("%0*d", indexWidth, index+1))
			newName = strings.ReplaceAll(newName, "{ext}", path.Ext(targetFile.OldName)[1:])

			oldPath := path.Join(targetFile.Dir, targetFile.OldName)
			newPath := path.Join(targetFile.Dir, newName)

			renameFiles = append(renameFiles, RenameFile{
				OldPath: oldPath,
				NewPath: newPath,
			})
		}
	}

	// リネーム元のパスの昇順でソート（リネーム差分の表示を分かりやすくするため）
	sort.SliceStable(renameFiles, func(i, j int) bool {
		return renameFiles[i].OldPath < renameFiles[j].OldPath
	})

	// リネーム内容の表示だけ行う
	if dryRun {
		fmt.Println("=== Dry Run ===")
		for _, renameFile := range renameFiles {
			fmt.Printf("%s -> %s\n", renameFile.OldPath, path.Base(renameFile.NewPath))
		}

		return nil
	}

	// リネーム前の最終確認
	if interactive {
		for _, renameFile := range renameFiles {
			fmt.Printf("%s -> %s\n", renameFile.OldPath, path.Base(renameFile.NewPath))
		}
		fmt.Print("上記の内容でリネームを実行しますか？ (y/n): ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("リネームを中止します")
			return nil
		}
	}

	// リネーム実行
	for _, renameFile := range renameFiles {
		fmt.Printf("Renaming: %s -> %s\n", renameFile.OldPath, path.Base(renameFile.NewPath))
		if err := os.Rename(renameFile.OldPath, renameFile.NewPath); err != nil {
			return xerrors.Errorf("ファイルのリネームに失敗: %w", err)
		}
	}

	return nil
}

func replaceFormatExceptIndexAndExt(format, oldName string, modTime time.Time) string {
	newName := format

	newName = strings.ReplaceAll(newName, "{old_name}", oldName)
	newName = strings.ReplaceAll(newName, "{year}", modTime.Format("2006"))
	newName = strings.ReplaceAll(newName, "{month}", modTime.Format("01"))
	newName = strings.ReplaceAll(newName, "{day}", modTime.Format("02"))
	newName = strings.ReplaceAll(newName, "{hour}", modTime.Format("15"))
	newName = strings.ReplaceAll(newName, "{minute}", modTime.Format("04"))
	newName = strings.ReplaceAll(newName, "{second}", modTime.Format("05"))

	return newName
}
