package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"html/template"
	"io"
	"os"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type fileMode string

const (
	modeText fileMode = "text"
	modeJSON fileMode = "json"
)

type lineOp struct {
	typeCode diffmatchpatch.Operation
	text     string
}

type diffRow struct {
	Kind      string
	Label     string
	LeftNum   int
	RightNum  int
	LeftHTML  template.HTML
	RightHTML template.HTML
}

type pageData struct {
	Title        string
	BeforePath   string
	AfterPath    string
	GeneratedAt  string
	TotalRows    int
	AddedCount   int
	DeletedCount int
	ChangedCount int
	Rows         []diffRow
}

func main() {
	beforePath := flag.String("before", "", "比較元ファイルのパス")
	afterPath := flag.String("after", "", "比較先ファイルのパス")
	outPath := flag.String("out", "diff.html", "出力先HTMLパス")
	title := flag.String("title", "テキスト差分レポート", "HTMLタイトル")
	modeStr := flag.String("mode", string(modeText), "比較モード: text または json")
	flag.Parse()

	if *beforePath == "" || *afterPath == "" {
		exitWithUsage("-before と -after は必須です")
	}

	mode := fileMode(*modeStr)
	if mode != modeText && mode != modeJSON {
		exitWithUsage("-mode は text または json を指定してください")
	}

	before, err := readInput(*beforePath, mode)
	if err != nil {
		die(err)
	}
	after, err := readInput(*afterPath, mode)
	if err != nil {
		die(err)
	}

	rows, add, del, changed := buildRows(before, after)

	if err := writeHTML(*outPath, pageData{
		Title:        *title,
		BeforePath:   *beforePath,
		AfterPath:    *afterPath,
		GeneratedAt:  time.Now().Format("2006-01-02 15:04:05"),
		TotalRows:    len(rows),
		AddedCount:   add,
		DeletedCount: del,
		ChangedCount: changed,
		Rows:         rows,
	}); err != nil {
		die(err)
	}

	fmt.Printf("HTML diff を出力しました: %s\n", *outPath)
}

func exitWithUsage(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	fmt.Fprintln(os.Stderr, "usage: go run . -before before.txt -after after.txt -out diff.html [-mode text|json]")
	os.Exit(1)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func readInput(path string, mode fileMode) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s の読み込みに失敗: %w", path, err)
	}
	if mode == modeText {
		return string(raw), nil
	}

	normalized, err := normalizeJSON(raw)
	if err != nil {
		return "", fmt.Errorf("%s のJSON解析に失敗: %w", path, err)
	}
	return normalized, nil
}

func normalizeJSON(raw []byte) (string, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	if err := dec.Decode(&v); err != nil {
		return "", err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return "", fmt.Errorf("複数のJSON値が含まれています")
	}

	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

func buildRows(before, after string) ([]diffRow, int, int, int) {
	ops := lineDiffOps(before, after)
	rows := make([]diffRow, 0, len(ops))
	leftNum := 1
	rightNum := 1
	added := 0
	deleted := 0
	changed := 0

	for i := 0; i < len(ops); {
		op := ops[i]

		if op.typeCode == diffmatchpatch.DiffDelete || op.typeCode == diffmatchpatch.DiffInsert {
			dels := make([]string, 0)
			ins := make([]string, 0)
			for i < len(ops) && ops[i].typeCode != diffmatchpatch.DiffEqual {
				switch ops[i].typeCode {
				case diffmatchpatch.DiffDelete:
					dels = append(dels, ops[i].text)
				case diffmatchpatch.DiffInsert:
					ins = append(ins, ops[i].text)
				}
				i++
			}

			pair := max(len(dels), len(ins))
			for j := range pair {
				hasDel := j < len(dels)
				hasIns := j < len(ins)
				row := diffRow{}

				switch {
				case hasDel && hasIns:
					leftHTML, rightHTML := inlineHighlight(dels[j], ins[j])
					row = diffRow{
						Kind:      "changed",
						Label:     "変更",
						LeftNum:   leftNum,
						RightNum:  rightNum,
						LeftHTML:  leftHTML,
						RightHTML: rightHTML,
					}
					leftNum++
					rightNum++
					changed++
				case hasDel:
					row = diffRow{
						Kind:      "deleted",
						Label:     "削除",
						LeftNum:   leftNum,
						LeftHTML:  template.HTML(html.EscapeString(dels[j])),
						RightHTML: template.HTML(""),
					}
					leftNum++
					deleted++
				case hasIns:
					row = diffRow{
						Kind:      "added",
						Label:     "追加",
						RightNum:  rightNum,
						LeftHTML:  template.HTML(""),
						RightHTML: template.HTML(html.EscapeString(ins[j])),
					}
					rightNum++
					added++
				}
				rows = append(rows, row)
			}
			continue
		}

		row := diffRow{
			Kind:      "equal",
			Label:     "同じ",
			LeftNum:   leftNum,
			RightNum:  rightNum,
			LeftHTML:  template.HTML(html.EscapeString(op.text)),
			RightHTML: template.HTML(html.EscapeString(op.text)),
		}
		leftNum++
		rightNum++
		rows = append(rows, row)
		i++
	}

	return rows, added, deleted, changed
}

func lineDiffOps(before, after string) []lineOp {
	dmp := diffmatchpatch.New()
	beforeChars, afterChars, lineArray := dmp.DiffLinesToChars(before, after)
	diffs := dmp.DiffMain(beforeChars, afterChars, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	ops := make([]lineOp, 0)
	for _, d := range diffs {
		for _, line := range splitLines(d.Text) {
			ops = append(ops, lineOp{typeCode: d.Type, text: line})
		}
	}
	return ops
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	if strings.HasSuffix(text, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func inlineHighlight(left, right string) (template.HTML, template.HTML) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(left, right, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	var leftBuf strings.Builder
	var rightBuf strings.Builder

	for _, d := range diffs {
		text := html.EscapeString(d.Text)
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			leftBuf.WriteString(text)
			rightBuf.WriteString(text)
		case diffmatchpatch.DiffDelete:
			leftBuf.WriteString(`<span class="inline-del">`)
			leftBuf.WriteString(text)
			leftBuf.WriteString(`</span>`)
		case diffmatchpatch.DiffInsert:
			rightBuf.WriteString(`<span class="inline-ins">`)
			rightBuf.WriteString(text)
			rightBuf.WriteString(`</span>`)
		}
	}

	return template.HTML(leftBuf.String()), template.HTML(rightBuf.String())
}

func writeHTML(path string, data pageData) error {
	tpl, err := template.New("page").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tpl.Execute(f, data); err != nil {
		return err
	}
	return nil
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}}</title>
  <style>
    :root {
      --bg: #f5f7fa;
      --paper: #ffffff;
      --ink: #0f1b2d;
      --muted: #5d6b82;
      --line: #dbe3ee;
      --add: #e7f9ed;
      --del: #ffeaea;
      --chg: #fff6db;
      --equal: #ffffff;
      --add-strong: #1f8f4e;
      --del-strong: #c73c3c;
      --shadow: 0 12px 30px rgba(15, 27, 45, 0.08);
    }

    * {
      box-sizing: border-box;
    }

    body {
      margin: 0;
      color: var(--ink);
      font-family: "Hiragino Sans", "Yu Gothic", "Meiryo", sans-serif;
      background: var(--bg);
      min-height: 100vh;
      padding: 24px;
    }

    .wrap {
      max-width: 1200px;
      margin: 0 auto;
      background: var(--paper);
      border: 1px solid #edf2f8;
      border-radius: 8px;
      box-shadow: var(--shadow);
      overflow: hidden;
    }

    .head {
      padding: 20px 24px;
      border-bottom: 1px solid var(--line);
      background: #ffffff;
    }

    h1 {
      margin: 0;
      font-size: 1.5rem;
      letter-spacing: 0;
    }

    .meta {
      margin-top: 10px;
      color: var(--muted);
      font-size: 0.92rem;
      line-height: 1.6;
    }

    .stats {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
      margin-top: 12px;
    }

    .chip {
      border-radius: 8px;
      font-size: 0.86rem;
      padding: 6px 10px;
      border: 1px solid var(--line);
      background: #f8fbff;
    }

    .chip.add { background: var(--add); color: #0f6b3b; }
    .chip.del { background: var(--del); color: #8f2f2f; }
    .chip.chg { background: var(--chg); color: #8f6d15; }

    .table-wrap {
      overflow: auto;
      max-height: calc(100vh - 260px);
    }

    table {
      width: 100%;
      border-collapse: separate;
      border-spacing: 0;
      min-width: 860px;
    }

    thead th {
      position: sticky;
      top: 0;
      z-index: 1;
      text-align: left;
      font-size: 0.88rem;
      letter-spacing: 0;
      color: var(--muted);
      background: #f4f8fd;
      border-bottom: 1px solid var(--line);
      padding: 10px 12px;
    }

    tbody td {
      border-bottom: 1px solid #edf2f8;
      vertical-align: top;
      padding: 8px 12px;
      font-family: "SFMono-Regular", "Menlo", "Consolas", monospace;
      font-size: 13px;
      line-height: 1.6;
      white-space: pre-wrap;
      word-break: break-word;
    }

    td.kind {
      width: 78px;
      color: var(--muted);
      font-family: "Hiragino Sans", "Yu Gothic", "Meiryo", sans-serif;
      font-weight: 700;
      white-space: nowrap;
    }

    td.no {
      width: 62px;
      color: var(--muted);
      text-align: right;
      user-select: none;
      background: #fafcff;
      font-family: "SFMono-Regular", "Menlo", "Consolas", monospace;
    }

    tr.equal td.content { background: var(--equal); }
    tr.added td.content.right { background: var(--add); }
    tr.deleted td.content.left { background: var(--del); }
    tr.changed td.content { background: var(--chg); }
    tr.added td.kind { color: #0f6b3b; background: var(--add); }
    tr.deleted td.kind { color: #8f2f2f; background: var(--del); }
    tr.changed td.kind { color: #8f6d15; background: var(--chg); }

    .inline-ins {
      background: rgba(31, 143, 78, 0.3);
      color: var(--add-strong);
      border-radius: 4px;
      padding: 0 2px;
    }

    .inline-del {
      background: rgba(199, 60, 60, 0.25);
      color: var(--del-strong);
      border-radius: 4px;
      padding: 0 2px;
      text-decoration: line-through;
      text-decoration-thickness: 1px;
    }

    @media (max-width: 768px) {
      body {
        padding: 10px;
      }

      .head {
        padding: 14px;
      }

      h1 {
        font-size: 1.25rem;
      }

      .table-wrap {
        max-height: none;
      }

      tbody td {
        font-size: 12px;
      }
    }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="head">
      <h1>{{.Title}}</h1>
      <div class="meta">
        比較元: {{.BeforePath}}<br>
        比較先: {{.AfterPath}}<br>
        生成日時: {{.GeneratedAt}}
      </div>
      <div class="stats">
        <span class="chip">総行数: {{.TotalRows}}</span>
        <span class="chip add">追加: {{.AddedCount}}</span>
        <span class="chip del">削除: {{.DeletedCount}}</span>
        <span class="chip chg">変更: {{.ChangedCount}}</span>
        <span class="chip">緑=追加 / 赤=削除 / 黄=変更</span>
      </div>
    </div>

    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th style="width: 78px;">種別</th>
            <th style="width: 62px;">Before</th>
            <th>変更前</th>
            <th style="width: 62px;">After</th>
            <th>変更後</th>
          </tr>
        </thead>
        <tbody>
          {{range .Rows}}
          <tr class="{{.Kind}}">
            <td class="kind">{{.Label}}</td>
            <td class="no">{{if .LeftNum}}{{.LeftNum}}{{end}}</td>
            <td class="content left">{{.LeftHTML}}</td>
            <td class="no">{{if .RightNum}}{{.RightNum}}{{end}}</td>
            <td class="content right">{{.RightHTML}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </div>
  </div>
</body>
</html>`
