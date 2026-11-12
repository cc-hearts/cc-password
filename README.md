# bipPass

一个自托管的零知识密码管理器。桌面客户端用 Go 写，界面是 mygo 原生控件，不嵌 webview，一个二进制文件跑起来。

账号是一组 24 个助记词。没有用户名，没有邮箱，没有「忘记密码」。助记词和口令只在本机出现，服务端从来没见过它们，也没法见到——它拿到的只有密文和几条路由用的元数据。

> 目前做到 M2（本地保险库），M3 及以后还没做。能用的见下面「现在能做什么」，没做的见「还没做」。

## 安全模型

**服务端零知识。** 服务端只存密文和 `id / vaultId / rev / updatedAt / deleted` 这几条路由元数据。标题、网址、用户名、分类、保险库名字全都是密文。服务端被拖库，拿到的是谁也读不了的一堆字节。

**密钥只在本机，只在内面。** 根密钥由助记词和口令派生，解锁时进内存，锁定时清零。它不落盘、不进日志、不进崩溃报告、不进命令行参数、不进环境变量。落盘的只有两样：`keyfile.json`（被口令包起来的根密钥）和 `vault.db`（密文）。

**认证失败就是整体失败。** 口令错、keyfile 被改、密文被翻一个 bit——结果都一样：打不开，一条都不显示。没有「部分解密」，没有「降级显示」，没有哪一条因为碰巧能解开就先给你看。

**不静默丢数据。** 同步冲突时两边副本都留着给你选，不做 last-write-wins。整库覆盖式同步是旧项目丢数据的根源。

**默认参数就是安全参数。** Argon2id 用 m=65536 KiB、t=3、p=1，没有「快速模式」，界面上也调不了。

**导出永远在。** 明文导出随时可用，按一下确认，再按一下确认。不被锁死在一个软件里是底线。

用到的都是标准件，没有自研协议、没有自研 KDF：BIP39、RFC 5869（HKDF-SHA256）、RFC 9106（Argon2id）、draft-irtf-cfrg-xchacha-03（XChaCha20-Poly1305）、RFC 8032（Ed25519）、RFC 6238（TOTP）、RFC 4648（base64url / Base32）。

## 现在能做什么

- 首次运行：生成 24 个助记词，设口令，落地 keyfile
- 解锁：口令 + keyfile 打开本地保险库；口令错和 keyfile 损坏是同一条提示
- 恢复：填 24 个词 + 设新口令，从助记词重建账号
- 条目：登录、笔记、信用卡、身份四类，增删改查、历史密码、自定义字段
- 保险库：多个保险库，条目按保险库分开，可新建
- 搜索：本地内存里搜标题和用户名，服务端不参与
- 生成器：20 位四类字符，可调长度和字符类，显示强度
- TOTP：粘贴 Base32 或 `otpauth://` 链接，显示 6 位验证码和倒计时
- 自动锁定：默认 5 分钟无操作，可选 1–60 分钟，没有「永不」
- 剪贴板：复制密码 30 秒后清除，清除前先比对是不是自己写进去的那份
- 锁定：窗口最小化不管，但休眠、锁屏、手动锁定都会立即清掉密钥和解密数据
- 数据存在本地，不上传任何地方

## 还没做

M3 及以后的都在计划里，没开工：

- 导入导出（1Password / Bitwarden JSON / Chrome CSV）
- 同步（自托管服务端 + item 级 oplog + 冲突副本）
- 浏览器扩展
- 手机端和生物识别（Touch ID / Hello，含免输口令）
- 团队、共享保险库、权限模型（明确不做）
- 区块链上链（明确不做：密文上链等于永久公开且删不掉）

## 构建与运行

需要 Go 1.27.3 或更高。mygo 的桌面后端是 CGo 的，所以桌面版要装对应的 C 工具链；`go build` 出来的二进制在 Linux 上能跑，macOS 上需要 Xcode 命令行工具。

```bash
go build -o bippass .
./bippass
```

```bash
go test ./...
go vet ./...
go run github.com/egoist/mygo/cmd/mygo vet
go run ./tools/vectors -check
```

`vectors -check` 跑的是外部权威向量：RFC 6238 附录 B、BIP39 官方向量、RFC 5869、draft-irtf-cfrg-xchacha-03 附录 A.3、Argon2id 参考实现。文件是提交进仓库的，实现改了而契约没更新，这一步会红。

## 数据放在哪

默认在用户配置目录下，跟系统走：

| 平台 | 位置 |
|---|---|
| macOS | `~/Library/Application Support/bipPass/` |
| Linux | `~/.config/bipPass/` |
| Windows | `%AppData%\bipPass\` |

里面两个文件：

- `keyfile.json` —— 账号 ID、Argon2id 参数、盐、nonce，以及被口令包起来的根密钥。权限 0600，写入是临时文件 + fsync + rename，崩在中间不会写坏原文件
- `vault.db` —— SQLite，WAL 模式，权限 0600。所有条目标题、用户名、网址、备注、保险库名字都是密文

想换个位置，设 `BIPPASS_HOME`：

```bash
BIPPASS_HOME=/Volumes/usb/vault ./bippass
```

目录会自动建，权限 0700。换目录等于换一份数据，两边不共享。

## 目录结构

```
internal/vaultcrypto/   助记词、密钥树、AEAD、口令包裹、条目封装、TOTP、生成器
internal/identity/      账号、keyfile 读写、解锁
internal/store/         SQLite：条目表、oplog、meta
internal/vault/         保险库引擎：增删改查、rev、墓碑、冲突副本
internal/ccapp/         桌面界面：解锁、恢复、首建、列表、详情、表单、生成器、设置、锁定
tools/vectors/          生成并校验 spec/crypto-vectors.json
spec/                   字节级契约：密钥派生、条目格式、外部向量
docs/                   方案文档和交接记录
```

`spec/key-hierarchy.md` 和 `spec/item-format.md` 是字节级契约，M6 的手机端和浏览器端实现照它对。

## 检查一下自己的库

想知道自己的库是不是真的加密了，可以自己看：

```bash
strings ~/Library/Application\ Support/bipPass/vault.db | grep -i 你的某个密码
```

搜不到就对了。这条在方案里是一条硬要求，不是宣传语。

## 许可

MIT，见 `LICENSE`。
