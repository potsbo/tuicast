# tuicast 仕様書

## 概要

tuicast は fzf ベースの TUI ランチャーフレームワーク。YAML で定義されたメニュー構造に従い、ユーザーにアイテムを選ばせてコマンドを実行する。

tuicast 自身はドメイン知識を持たない。セッション管理、リポジトリ、SSH ホストなどの概念はすべて外部コマンド側の責務。tuicast は「行を集めて、選ばせて、実行する」だけのツール。

## 設計原則

- fzf をフィルタリングエンジンとして使う。fuzzy filter の自前実装はしない
- テンプレートエンジンを自前実装しない。変数展開はシェルの環境変数 (`$name`) をそのまま使う
- tuicast が持つ概念は 3 種類の view と 2 種類の form step のみ

## CLI

```
tuicast                          # デフォルト config を読んで views.main を開く
tuicast -c path/to/config.yaml   # 指定した config を読む
tuicast --view <name>            # 指定した view を直接開く (デバッグ用)
```

設定ファイルのデフォルトパス: `~/.config/tuicast/config.yaml`

## Config の構造

Config はトップレベルに `views` を持ち、各 view は名前で参照される。

```yaml
views:
  <view_name>: <View>
  <view_name>: <View>
  ...
```

### View

View は以下の 3 種類のいずれか。どのフィールドを持つかで判別する。

**FormView** — 0 回以上の選択ステップを経てコマンドを実行する:

| フィールド | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `title` | no | string | 表示名。menu から参照されたときに使われる |
| `form` | no | []FormStep | 選択ステップの配列。省略時は 0 ステップ (leaf) |
| `run` | yes | ShellCommand | 最後に実行するコマンド |

**UnionView** — 複数の FormView のアイテムをフラットに結合する:

| フィールド | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `title` | no | string | 表示名 |
| `union` | yes | []ViewName | 参照する FormView の名前。FormView かつ 1 ステップのもののみ |

**MenuView** — view の title を一覧表示し、選択された view に遷移する:

| フィールド | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `title` | no | string | 表示名 |
| `menu` | yes | []ViewName | 参照する view の名前。任意の種類の view を参照可能 |

### FormStep

FormStep は以下の 2 種類のいずれか。`list` を持てば SelectStep、`placeholder` を持てば InputStep。両方持つ、または両方持たない FormStep は不正。

**SelectStep** — コマンドの出力から 1 行を選ばせる:

| フィールド | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `name` | yes | StepName | 結果を格納する環境変数名 |
| `list` | yes | ShellCommand | 選択肢を生成するコマンド。stdout の各行が 1 アイテム |
| `display` | no | TransformCommand | list の出力を表示用に変換するコマンド |
| `preview` | no | TransformCommand | フォーカス中のアイテムの詳細を表示するコマンド |

**InputStep** — テキストを自由入力させる:

| フィールド | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `name` | yes | StepName | 結果を格納する環境変数名 |
| `placeholder` | yes | string | 入力欄に表示するヒントテキスト |

### 値の型

**ViewName**: Config 内の `views` に定義された view 名を参照する文字列。存在しない名前はバリデーションエラー。

**StepName**: 環境変数名として有効な文字列 (`[a-zA-Z_][a-zA-Z0-9_]*`)。form 内の後続ステップや `run` から `$name` で参照される。

**ShellCommand**: `sh -c` で実行されるシェルコマンド文字列。前ステップの結果を `$name` で参照可能。

**TransformCommand**: display と preview で共通の変換コマンド文字列。実行モードは記法で決まる:
- `{}` を含む場合 → **per-item モード**: `{}` がアイテムに置換され、アイテムごとにコマンドが実行される
- `|` で始まる場合 → **pipe モード**: stdin から全行受け取り、stdout で全行返す (行数一致必須)
- どちらにも該当しない場合 → config 読み込み時にバリデーションエラー

## View の種類

### FormView

0 回以上の選択ステップを経て、最後にコマンドを実行する。

#### 0 ステップ (leaf)

form を持たない。即座にコマンドを実行する。変数を参照できないので、`run` は固定のコマンドになる。menu から遷移して使う。

```yaml
open-lazygit:
  title: Lazygit
  run: lazygit
```

#### 1 ステップ

一覧から選んで実行する。

```yaml
files:
  title: Files
  form:
    - name: file
      list: find . -type f
  run: vim $file
```

#### n ステップ (wizard)

複数回選んで最後に実行する。

```yaml
checkout:
  title: Checkout remote branch
  form:
    - name: remote
      list: git remote
    - name: branch
      list: git branch -r --format=%(refname:short) --list "$remote/*"
      preview: git log --oneline -20 {}
  run: git checkout --track $branch
```

### UnionView

複数の FormView のアイテムをフラットに結合して 1 つのリストとして表示する。各アイテムの実行コマンドは元の FormView の `run` に従う。

```yaml
main:
  union: [sessions, repos, ssh, commands]
```

制約:
- 参照先は FormView のみ (MenuView, UnionView は不可)
- 参照先の FormView はちょうど 1 ステップであること (0 ステップはアイテムを持たず、2 ステップ以上は union 内で完結できないため)

### MenuView

参照された view の `title` を一覧表示し、選択された view に遷移する。

```yaml
main:
  menu: [sessions, repos, ssh, commands, new-worktree]
```

- 任意の view (FormView, UnionView, MenuView) を参照できる
- 表示には各 view の `title` を使う。`title` がない場合は view 名をそのまま表示する

## FormStep の詳細

SelectStep と InputStep の仕様は「Config の構造」セクションのテーブルを参照。

## TransformCommand

display と preview で共通の型。実行モードは記法で決まる。

### per-item モード

`{}` を含む場合。`{}` がアイテムに置換され、アイテムごとにコマンドが実行される。

```yaml
display: basename {}
preview: ls -la {}
preview: "cat {} || echo 'no preview'"
```

preview の `{}` は fzf の `--preview` にそのまま渡される。fzf がフォーカス中のアイテムに展開する。

display の `{}` は tuicast が各行に対して展開・実行し、結果を収集する。

### pipe モード

`|` で始まる場合。list の全出力を stdin で受け取り、変換後の全行を stdout で返す。

```yaml
display: "| sed 's|.*/||'"
```

stdin の行数と stdout の行数は一致すること。行の対応は順序で決まる。

## 変数の仕組み

テンプレートエンジンは使わない。シェルの環境変数をそのまま利用する。

tuicast は各 form step の結果を環境変数としてセットし、後続のコマンドを `sh -c` で実行する。

```
Step 1: ユーザーが remote を選択 → export remote="origin"
Step 2: sh -c 'git branch -r --list "$remote/*"' を実行
Step 2: ユーザーが branch を選択 → export branch="origin/feature-x"
Run:    sh -c 'git checkout --track $branch' を実行
```

変数のスコープ:
- `list`: それより前のステップの変数を参照可能
- `display` / `preview`: 同上。加えて `{}` で現在のアイテムを参照
- `run`: 全ステップの変数を参照可能

## 実行モデル

### FormView の実行

```
env = {}

for each step in form:
    if step is SelectStep:
        lines = sh(step.list, env)
        if step.display:
            display_lines = transform(step.display, lines)
        else:
            display_lines = lines
        selected_index = fzf(display_lines, preview=step.preview)
        env[step.name] = lines[selected_index]  # 生の行を格納
    if step is InputStep:
        value = text_input(step.placeholder)
        env[step.name] = value

sh(run, env)
```

display の適用後も、`$name` に格納されるのは `list` の生の行。display はあくまで表示用。

### UnionView の実行

複数の FormView のアイテムを結合する。各アイテムがどの FormView 由来かを識別するため、fzf に渡す行にメタデータを埋め込む。

```
all_lines = []

for each view_name in union:
    view = views[view_name]
    step = view.form[0]
    lines = sh(step.list)
    if step.display:
        display_lines = transform(step.display, lines)
    else:
        display_lines = lines
    for i, line in lines:
        all_lines.append("{view_name}\t{line}\t{display_lines[i]}")

# preview ディスパッチ:
# 各 view の preview コマンドが異なるため、tuicast がラッパーを生成する。
# ラッパーは view_name (フィールド1) を見て対応する preview コマンドを実行する。
# preview を持たない view のアイテムにはプレビューを表示しない。
preview_script = generate_preview_dispatcher(union, views)

# fzf に渡す:
#   --delimiter '\t'
#   --with-nth 3    (表示テキストのみ表示)
#   --preview preview_script
selected = fzf(all_lines)

# パース:
view_name = selected.split('\t')[0]
raw_line = selected.split('\t')[1]

# 元の view の run を実行:
view = views[view_name]
env = { view.form[0].name: raw_line }
sh(view.run, env)
```

### MenuView の実行

```
items = []
for each view_name in menu:
    title = views[view_name].title ?? view_name
    items.append("{view_name}\t{title}")

selected = fzf(items, --with-nth 2)
target = selected.split('\t')[0]

execute_view(target)
```

menu から遷移した先の view で Escape を押すと menu に戻る。外側の fzf が `execute(...)` で内側の fzf を呼び出し、内側が終了すると外側に戻る。

### TransformCommand の実行

```
function transform(cmd, lines):
    if cmd starts with "|":
        pipe_cmd = cmd[1:].trim()
        return sh_pipe(lines, pipe_cmd)   # echo lines | pipe_cmd
    else:
        result = []
        for line in lines:
            expanded = cmd.replace("{}", line)
            result.append(sh(expanded))
        return result
```

## 履歴

tuicast は FormView の `run` を実行するたびに、展開済みのコマンドを履歴ファイルに追記する。

ファイルパス: `~/.local/state/tuicast/history`

フォーマット: 1 行 1 コマンド (展開済み)

```
vim ./src/main.go
ssh server-1
git checkout --track origin/feature-x
lazygit
```

この履歴ファイルを `list` で読む view を定義すれば、最近の操作を再実行できる:

```yaml
history:
  title: Recent
  form:
    - name: cmd
      list: tac ~/.local/state/tuicast/history
  run: $cmd
```

## エラーハンドリング

### config 読み込み時

- `union` が FormView 以外を参照: エラー
- `union` が 2 ステップ以上の FormView を参照: エラー
- `menu` / `union` が存在しない view 名を参照: エラー
- FormStep に `list` も `placeholder` もない: エラー
- FormStep に `name` がない: エラー

### 実行時

- `list` コマンドが非ゼロで終了: エラーメッセージを表示して view を閉じる
- pipe モードの `display` で出力行数が `list` と不一致: エラー
- `run` コマンドが非ゼロで終了: エラーメッセージを表示 (tuicast 自体は正常終了)
- fzf で Escape: 現在の view を閉じる (menu なら親に戻る、最上位なら tuicast 終了)
- form の途中で Escape: その form を中断して前の画面に戻る

## 設定例

### 最小例

```yaml
views:
  main:
    form:
      - name: file
        list: find . -type f
    run: vim $file
```

### union を使った例

```yaml
views:
  main:
    union: [files, branches, commands]

  files:
    title: Files
    form:
      - name: file
        list: find . -type f -not -path './.git/*'
        display: basename {}
        preview: head -50 {}
    run: vim $file

  branches:
    title: Branches
    form:
      - name: branch
        list: git branch --format=%(refname:short)
        preview: git log --oneline -10 {}
    run: git switch $branch

  commands:
    title: Commands
    form:
      - name: cmd
        list: "echo -e 'lazygit\nnvim\nclaude'"
    run: $cmd
```

### n ステップ form の例

```yaml
views:
  main:
    menu: [files, checkout, history]

  files:
    title: Files
    form:
      - name: file
        list: find . -type f -not -path './.git/*'
        preview: head -50 {}
    run: vim $file

  checkout:
    title: Checkout remote branch
    form:
      - name: remote
        list: git remote
      - name: branch
        list: git branch -r --format=%(refname:short) --list "$remote/*"
        preview: git log --oneline -20 {}
    run: git checkout --track $branch

  history:
    title: Recent
    form:
      - name: cmd
        list: tac ~/.local/state/tuicast/history
    run: $cmd
```

## 実装

- 言語: Go
- fzf は外部プロセスとして呼び出す
- menu のネスト (push/pop) は fzf の `execute(...)` バインドで実現する
