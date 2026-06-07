# raku: GitHub PRコメントの既読差分管理ツール 仕様メモ

## 1. rakuのコンセプト

`raku` は、GitHub Pull Request に届いたコメントを「確認タスク」として扱うための、自分専用の薄いCLIツールである。

このツールの本質は、GitHub APIクライアントを自前で作り込むことではなく、`gh` コマンドで取得できるPRコメント群に対して、**既読・未読・更新ありの差分管理**を行うことである。

役割分担は次のようにする。

```txt
gh
  GitHubからPR情報・コメント・レビューコメント・レビュー本体を取得する係

raku
  取得したコメントを正規化する係
  自分のコメントを除外する係
  前回のseen stateと比較する係
  新規・更新ありコメントだけを表示する係
  表示番号とGitHub上の安定IDを紐づける係

Codex
  rakuが表示したコメントを読んで、返信案・修正方針・対応優先度を考える係
```

`raku` は、GitHubのPRコメントをそのまま「コメント一覧」として扱うのではなく、ユーザーにとっての **PRコメント確認タスクのInbox** として扱う。

そのため、ユーザーはGitHub上のコメントIDを意識しない。

```txt
GitHub側の安定ID
  review_comment:123456
  issue_comment:777888
  review:999000

raku側の表示番号
  [1]
  [2]
  [3]
```

ユーザーが意識するのは、直近の `raku get pr-comment` で表示された番号だけでよい。

```bash
raku get pr-comment
```

で新規・更新ありコメントを表示し、

```bash
raku seen pr-comment 1
```

で「表示された1番目のコメント確認タスクを対応済みにする」ことができる。

この `1` は永続IDではなく、**直近の表示結果に対する一時的なエイリアス**である。

---

## 2. コマンド体系

### 基本コマンド

```bash
raku get pr-comment
```

現在のリポジトリ・現在のブランチに紐づく、自分が作成したPRについて、新規または更新されたコメントだけを表示する。

表示後、各コメントには `[1]`, `[2]`, `[3]` のような番号を振る。

このコマンドは **seen stateを変更しない**。

---

```bash
raku get pr-comment --all
```

現在PR上に存在するコメントを、seen済みかどうかに関係なくすべて表示する。

初回確認やデバッグに使う。

このコマンドも **seen stateを変更しない**。

---

```bash
raku get pr-comment --json
```

新規または更新されたコメントをJSON形式で表示する。

Codexや他のツールに渡す用途を想定する。

このコマンドも **seen stateを変更しない**。

---

```bash
raku get pr-comment --all --json
```

現在PR上に存在するコメントを、seen済みかどうかに関係なくすべてJSON形式で表示する。

---

### seenコマンド

```bash
raku seen pr-comment 1
```

直近の `raku get pr-comment` で表示された `[1]` のコメント確認タスクをseenにする。

---

```bash
raku seen pr-comment 1 3
```

直近の表示結果の `[1]` と `[3]` をseenにする。

---

```bash
raku seen pr-comment 1-3
```

直近の表示結果の `[1]` から `[3]` までをseenにする。

---

```bash
raku seen pr-comment --all
```

直近の `raku get pr-comment` で表示されたコメント確認タスクをすべてseenにする。

ここでの `--all` は、**直近表示された一覧すべて**という意味である。

---

```bash
raku seen pr-comment --baseline
```

現在PR上に存在するすべてのコメントをseen扱いにする。

初回セットアップ用。

すでにコメントが大量に付いているPRで、今あるものをすべて「確認済みの基準」にしたい場合に使う。

---

### デバッグ・管理コマンド候補

```bash
raku status pr-comment
```

現在PRについて、seen済み件数、未seen件数、直近表示ビューの有無などを表示する。

---

```bash
raku reset pr-comment
```

現在PRに関するrakuのstateを削除する。

デバッグ用。

---

```bash
raku config
```

将来的に、通知先、bot除外設定、state保存先、JSON出力形式などを設定するために使う。

ただし、MVPでは必須ではない。

現在リポジトリ・現在ブランチのPRだけを見るなら、設定ファイルなしで始められる。

---

## 3. コマンドが達成すること

### `raku get pr-comment` が達成すること

`raku get pr-comment` は、次の処理を行う。

```txt
1. 現在のrepoを特定する
2. 現在のbranchに紐づくPRを特定する
3. PR authorが自分か確認する
4. PRに付いたコメント・レビューコメント・レビュー本体を全部取る
5. 自分のコメントを除外する
6. 前回seen stateと比較する
7. 新規または更新されたものだけ表示する
8. 表示番号を振る
9. 表示番号とGitHub上の安定IDの対応表をcurrent_viewとして保存する
10. seen stateは変更しない
```

対象にするPRは、現在作業しているGitリポジトリの、現在チェックアウトしているブランチに紐づくPRである。

PRのauthorが自分ではない場合、コメント収集は行わない。

---

### 取得対象のコメント種別

PRコメントには大きく3種類ある。

```txt
issue_comment
  PR Conversation に付く普通のコメント
  ファイルや行番号には基本的に紐づかない

review_comment
  Files changed 上の特定行に付くレビューコメント
  path / line / side / diff_hunk などの位置情報を持つ

review
  APPROVED / CHANGES_REQUESTED / COMMENTED などのレビュー本体
  ファイルや行番号には基本的に紐づかない
```

`raku` はこの3種類をすべて取得し、内部的に共通のevent形式へ正規化する。

---

### `review_comment` の位置情報

Codexと会話するためには、どのファイルのどの行にコメントが来たかが重要である。

そのため、`review_comment` では少なくとも次の情報を保持する。

```json
{
  "type": "review_comment",
  "id": 123456,
  "author": "reviewer-a",
  "body": "ここ nil の可能性があります",
  "location": {
    "path": "internal/user/profile.go",
    "line": 87,
    "start_line": null,
    "side": "RIGHT",
    "start_side": null,
    "diff_hunk": "@@ -80,10 +80,14 @@ func validateProfile(...)"
  }
}
```

この情報があれば、Codexに対して次のような依頼ができる。

```txt
このreview_commentに対して返信案を作って。
必要なら該当ファイルの該当行周辺を見て、最小修正方針も出して。
```

---

### `raku get pr-comment --all` が達成すること

`raku get pr-comment --all` は、seen stateを無視して、現在PR上に存在するすべてのコメントを表示する。

用途は次の通り。

```txt
- 初回に全体を見たい
- seen済みコメントも含めて確認したい
- デバッグしたい
- Codexに全コメント文脈を渡したい
```

このコマンドも表示番号を振り、`current_view` を保存する。

ただし、seen stateは変更しない。

---

### `raku seen pr-comment 1` が達成すること

`raku seen pr-comment 1` は、直近の `raku get pr-comment` で表示された `[1]` をseenにする。

内部的には、`current_view` に保存されている番号対応表を使う。

```txt
[1]
  ↓ current_viewで解決
review_comment:123456
  ↓ seenに保存
seen[owner/repo#123][review_comment:123456] = fingerprint
```

つまり、ユーザーはGitHub上のコメントIDを覚えなくてよい。

---

### `raku seen pr-comment --all` が達成すること

直近の `raku get pr-comment` で表示された一覧をすべてseenにする。

これは、今回の確認タスクをすべて片付けたときに使う。

```bash
raku get pr-comment
# [1], [2], [3] が表示される

# 全部確認・対応した
raku seen pr-comment --all
```

---

### `raku seen pr-comment --baseline` が達成すること

現在PR上に存在する全コメントをseen扱いにする。

これは初回セットアップ用である。

```bash
raku seen pr-comment --baseline
```

を実行すると、現在のコメント状態が基準になる。

その後、

```bash
raku get pr-comment
```

を実行したときには、baseline以降に新しく来たコメント、または編集されたコメントだけが表示される。

---

## 4. そのための開発方針

### 方針1: `gh` を薄くラップする

GitHub API通信や認証は自前実装しない。

基本的には `gh` コマンドを子プロセスとして呼び出す。

現在ブランチのPR取得。

```bash
gh pr view --json number,title,url,author,headRefName,baseRefName,headRefOid
```

ログインユーザー取得。

```bash
gh api user --jq '.login'
```

PR Conversationコメント取得。

```bash
gh api "repos/{owner}/{repo}/issues/${pr_number}/comments" --paginate
```

Files changed上のレビューコメント取得。

```bash
gh api "repos/{owner}/{repo}/pulls/${pr_number}/comments" --paginate
```

レビュー本体取得。

```bash
gh api "repos/{owner}/{repo}/pulls/${pr_number}/reviews" --paginate
```

`repos/{owner}/{repo}` の `{owner}` と `{repo}` は、`gh api` が現在のリポジトリから補完できるため、現在作業中のrepoを対象にしやすい。

---

### 方針2: stateはrepo内に置かない

`raku` のseen stateは個人の作業状態であり、Git管理されるべきではない。

そのため、保存先はリポジトリ配下ではなく、ユーザーのローカルstate領域に置く。

第一候補は次の通り。

```txt
~/.local/state/raku/state.json
```

必要になったら、PRごとに分割してもよい。

```txt
~/.local/state/raku/repos/owner__repo/pr-123.json
```

ただしMVPでは、単一の `state.json` で十分。

---

### 方針3: stateのキーはbranch名ではなくPR番号にする

branch名は削除・再利用される可能性がある。

一方でPR番号はrepo内で安定している。

そのため、seen stateのキーは次の形式にする。

```txt
owner/repo#123
```

内部構造例。

```json
{
  "version": 1,
  "seen": {
    "owner/repo#123": {
      "review_comment:123456": {
        "fingerprint": "sha256:aaa",
        "seen_at": "2026-06-06T12:10:00Z"
      },
      "issue_comment:777888": {
        "fingerprint": "sha256:bbb",
        "seen_at": "2026-06-06T12:20:00Z"
      }
    }
  }
}
```

---

### 方針4: 差分判定は `event_key + fingerprint` で行う

各コメントには、GitHub上の安定IDを元にした `event_key` を付ける。

```txt
issue_comment:111
review_comment:222
review:333
```

さらに、コメント本文や位置情報など、rakuが重要視するフィールドからfingerprintを作る。

```txt
fingerprint = sha256(normalized important fields)
```

差分判定は次のようにする。

```txt
seenにevent_keyがない
  => new

seenにevent_keyはあるがfingerprintが違う
  => updated

seenにevent_keyがありfingerprintも同じ
  => seen
```

---

### 方針5: コメント本文の編集を再表示する

コメント本文が編集された場合、GitHub上のコメントIDは変わらない。

```txt
review_comment:222
```

しかし、`body` が変わればfingerprintが変わる。

```txt
前回 fingerprint: sha256:aaa
今回 fingerprint: sha256:bbb
```

そのため、過去にseen済みのコメントでも、本文が編集された場合は `updated` として再表示する。

---

### 方針6: fingerprintに入れるフィールド

MVPでは、次のフィールドからfingerprintを作る。

```txt
issue_comment:
  type
  id
  author
  body

review_comment:
  type
  id
  author
  body
  path
  line
  start_line
  side
  start_side
  diff_hunk

review:
  type
  id
  author
  state
  body
```

`created_at` や `updated_at` は表示用に保持するが、fingerprintに含めるかは慎重に決める。

MVPでは、重要フィールドが変わったときだけ再表示したいので、`updated_at` はfingerprintに含めない方針がよい。

---

### 方針7: `get` は非破壊にする

`raku get pr-comment` は、コメントを表示するだけで、seen stateを変更しない。

理由は次の通り。

```txt
- 表示しただけで対応済みにしたくない
- Codex処理が失敗した場合に取りこぼしたくない
- ユーザーが解決したものから順にseenにしたい
```

seenにするのは、明示的に次のコマンドを実行したときだけ。

```bash
raku seen pr-comment 1
raku seen pr-comment --all
raku seen pr-comment --baseline
```

---

### 方針8: current_viewを保存する

`raku get pr-comment` は、表示結果に番号を振ったうえで、その番号とGitHub上の安定IDの対応表を保存する。

保存例。

```json
{
  "current_view": {
    "owner/repo#123": {
      "created_at": "2026-06-06T12:30:00Z",
      "items": [
        {
          "alias": 1,
          "key": "review_comment:123456",
          "fingerprint": "sha256:aaa"
        },
        {
          "alias": 2,
          "key": "issue_comment:777888",
          "fingerprint": "sha256:bbb"
        }
      ]
    }
  }
}
```

`raku seen pr-comment 1` は、この `current_view` を使って `[1]` を解決する。

---

### 方針9: 表示番号は永続IDではない

`[1]`, `[2]`, `[3]` は、直近の `raku get pr-comment` の表示結果に対する一時的な番号である。

つまり、次のようなルールにする。

```txt
番号は直近の raku get pr-comment の表示結果に対応しています。
```

このルールをヘルプや出力に軽く表示しておくとよい。

---

### 方針10: 出力は人間向けとCodex向けを分ける

人間向けの通常出力例。

```txt
PR #123 Fix user validation
https://github.com/owner/repo/pull/123

[1] new review_comment by reviewer-a
    file: internal/user/profile.go:87
    side: RIGHT
    url: https://github.com/owner/repo/pull/123#discussion_r123456

    ここ nil の可能性があります

[2] new issue_comment by reviewer-b
    location: Conversation
    url: https://github.com/owner/repo/pull/123#issuecomment-777888

    この仕様で問題ないですか？

Tip: 番号は直近の raku get pr-comment の表示結果に対応しています。
     対応済みにするには: raku seen pr-comment 1
```

Codex向けJSON出力例。

```json
{
  "repo": "owner/repo",
  "pr": {
    "number": 123,
    "title": "Fix user validation",
    "url": "https://github.com/owner/repo/pull/123",
    "author": "me",
    "head_ref": "feature/user-validation",
    "base_ref": "main"
  },
  "mode": "unread",
  "events": [
    {
      "alias": 1,
      "status": "new",
      "key": "review_comment:123456",
      "type": "review_comment",
      "id": 123456,
      "author": "reviewer-a",
      "created_at": "2026-06-06T10:00:00Z",
      "updated_at": "2026-06-06T10:00:00Z",
      "body": "ここ nil の可能性があります",
      "url": "https://github.com/owner/repo/pull/123#discussion_r123456",
      "location": {
        "path": "internal/user/profile.go",
        "line": 87,
        "start_line": null,
        "side": "RIGHT",
        "start_side": null,
        "diff_hunk": "@@ -80,10 +80,14 @@ func validateProfile(...)"
      }
    }
  ]
}
```

---

### 方針11: 初回挙動はユーザーに選ばせる

stateが存在しないPRで `raku get pr-comment` を実行した場合、過去コメントを大量に表示してよいかは微妙である。

そのため、初回は次のような表示にする。

```txt
No raku state for owner/repo#123.

Current PR has:
- 3 conversation comments
- 8 review line comments
- 2 reviews

Use:
  raku get pr-comment --all
    show all comments

  raku seen pr-comment --baseline
    mark current comments as seen baseline
```

これにより、ユーザーは次を選べる。

```txt
全部見たい
  => raku get pr-comment --all

今あるものは見たことにして、次から新着だけ見たい
  => raku seen pr-comment --baseline
```

---

### 方針12: MVPでやること

MVPで作るべき範囲は次の通り。

```txt
- gh auth済み前提
- 現在repo・現在branchのPRを対象にする
- PR authorが自分か確認する
- issue_comment / review_comment / review を取得する
- 自分のコメントを除外する
- event_keyを作る
- fingerprintを作る
- state.jsonにseenを保存する
- current_viewを保存する
- raku get pr-comment
- raku get pr-comment --all
- raku get pr-comment --json
- raku seen pr-comment 1
- raku seen pr-comment 1 3
- raku seen pr-comment 1-3
- raku seen pr-comment --all
- raku seen pr-comment --baseline
```

---

### 方針13: MVPでは後回しにすること

最初から入れなくてよいもの。

```txt
- 30分ごとの自動実行
- 通知機能
- Codex自動呼び出し
- 自動返信投稿
- bot除外の細かい設定
- resolved / unresolved の厳密判定
- commit comments の取得
- 複数repo横断監視
- GitHub APIクライアントの自前実装
```

まずは、現在PRについて手動で次の流れができればよい。

```bash
raku get pr-comment
# [1], [2], [3] を見る

# 1番を対応した
raku seen pr-comment 1

# 残りを見る
raku get pr-comment

# 全部対応した
raku seen pr-comment --all
```

---

### 方針14: 将来のCodex連携

`raku get pr-comment --json` の出力をCodexに渡すことで、返信案や修正方針を作れるようにする。

想定フロー。

```bash
raku get pr-comment --json
```

Codexに依頼する内容。

```txt
このPRコメント確認タスクを読んで、各コメントについて以下を出して。

- 要約
- 返信案
- 修正が必要か
- 修正方針
- 優先度
- 追加で確認すべきファイル

自動でGitHubに投稿しないで。
```

対応後に、ユーザーが明示的にseenにする。

```bash
raku seen pr-comment 1
```

自動返信や自動修正は、MVPではやらない。

---

### 方針15: 実装言語

Bash + jqでも試作できるが、state管理とcurrent_viewを扱うなら、Go / Rust / Node.js / Pythonのいずれかがよい。

自分専用CLIとしてはGoが相性がよい。

理由は次の通り。

```txt
- 単一バイナリ化しやすい
- ghをexec.Commandで呼びやすい
- JSONの読み書きがしやすい
- 後からlaunchd/systemd対応を足しやすい
- 配布やインストールが簡単
```

ただし、MVPの価値は言語選定ではなく、次の体験を作ることにある。

```txt
新規コメントを見る
↓
番号で扱う
↓
対応したものからseenにする
↓
次回から出ない
↓
編集されたらupdatedとして再表示される
```

---

## まとめ

`raku` は、GitHub PRコメントを「新着確認タスク」として扱うための薄いCLIである。

最重要の設計は次の4つ。

```txt
1. getは非破壊
2. seenは明示操作
3. GitHub上の安定IDは裏で管理し、ユーザーには表示番号を渡す
4. 差分判定はevent_key + fingerprintで行う
```

この設計により、ユーザーは次のような自然な操作ができる。

```bash
raku get pr-comment
# どれどれ、俺に来たコメントの1番目は...

raku seen pr-comment 1
# 1番は対応した

raku get pr-comment
# まだ未対応のものだけ見る
```

`raku` は大きなGitHub連携ツールではなく、あくまで自分のPRコメント対応を楽にするための、既読差分管理付きInboxである。
