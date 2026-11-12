# ccpass 交接文档（M1/M2 剩余任务）

> 2026-11-12。需求以 `docs/plan.md`（已修订）为准；本文只记录进度、已定的实现细节、剩余任务的拆分。

## 1. 进度

分支 `ccpass-v1`（从 `master` 的 56f10db 分出，未推送）：

| 提交 | 内容 |
|---|---|
| 2a9e59d | 删掉旧 Electron 项目，建空 Go module `ccpass`，提交修订后的方案 |
| d430a30 | `vaultcrypto`：BIP39 24 词（生成、粘贴归一化、三类错误、seed） |
| 5461bb3 | `vaultcrypto`：密钥树、XChaCha20-Poly1305、Argon2id 包裹 root |
| 454f369 | `vaultcrypto`：条目与保险库名的加密封装（AAD 含 rev / updatedAt / deleted） |

没做：Task 5–18、整分支评审、CI。

### `internal/vaultcrypto` 已有导出

- **BIP39**：`MnemonicWords`、`GenerateMnemonic(r)`、`MnemonicFromEntropy(32 字节)`、`NormalizeMnemonic`、`ValidateMnemonic`、`MnemonicSeed(mnemonic, passphrase)`。错误类型是 `*MnemonicError{Err, Words, Position}`，`Err` 为 `ErrMnemonicLength / ErrMnemonicWord / ErrMnemonicChecksum`。错误文本只含词数和词位，不含词。归一化：NFKD + 小写 + 去掉不可见格式字符（零宽空格、BOM、软连字符）+ 所有空白折成一个空格
- **密钥树**：`KeySize`、`IDSize`、`RootKey(seed)`、`IdentityKey(root) ed25519.PrivateKey`、`AccountID(pub)`、`DefaultVaultID(root)`、`VaultKey(root, vaultID)`、`NewID(r)`、`EncodeB64`、`DecodeB64`（只认规范 base64url：无填充、无换行、无多余尾位）
- **AEAD**：`NonceSize`、`Overhead`、`ErrAuth`、`Seal(key, pt, aad, r)`、`Open(key, nonce, ct, aad)`。任何失败都是 `ErrAuth`，且不返回部分明文
- **口令包裹**：`SaltSize`、`Argon2Params{MKiB, T, P}`、`DefaultArgon2`、`ErrArgon2Params`、`(Argon2Params).Check()`（低于默认值或超过 4 GiB / 64 轮 / 16 线程都拒绝）、`DeriveKEK`、`Wrapped{Params, Salt, Nonce, CT}`
  - `WrapRoot(root, pass, accountID, p, r)`：从 r 先读 16 字节 salt，再读 24 字节 nonce
  - `UnwrapRoot`：参数越界返回 `ErrArgon2Params`，此时不跑 Argon2；其余失败一律 `ErrAuth`
- **封装**：`ErrHeader`、`ItemHeader{AccountID, VaultID, ItemID, Rev, UpdatedAt, Deleted}`、`VaultMetaHeader{AccountID, VaultID, Rev, UpdatedAt}`、`.AAD()`、`SealItem / OpenItem / SealVaultMeta / OpenVaultMeta`
  - header 要求：ID 是非空 base64url，rev ≥ 1，updatedAt ≥ 0。不满足时封装返回 `ErrHeader`，解封返回 `ErrAuth`
  - 空 payload 表示墓碑

### 测试里的独立向量

预期值都是字面量，用 Node `crypto`（OpenSSL）或 Python 独立算出，不是被测代码算的。Task 6 写 `spec/crypto-vectors.json` 时直接从这些测试里搬：

- `kdf_test.go`
  - RFC 5869 A.1–A.3
  - 零熵助记词（abandon ×23 + art，无第 25 词）派生出的 seed、root、身份种子、公钥、vault_key
  - 同一助记词的 account_id `8EEBT8Enxqe6rGiecvQliuOnMqpxjsePVGtu6fxuHtc`、默认保险库 id `s6WU001xGMDVUu6lu5mSNA`
- `aead_test.go`：draft-irtf-cfrg-xchacha-03 A.3.1
- `wrap_test.go`
  - Argon2id 参考向量（password / somesalt，t=2，m=65536，p=1）
  - 包裹结果：口令 `correct horse battery staple`，salt 为字节 00..0f，nonce 为字节 10..27
- `envelope_test.go`：条目与保险库名封装的 AAD 字面量和密文
- `bip39_test.go`：BIP39 官方 8 个英文 24 词向量（含口令 "TREZOR" 的 seed），以及词表的 SHA-256

## 2. 约定

- **module 与依赖**：module 名 `ccpass`，Go 1.27.3；已用 `golang.org/x/crypto v0.57.0`、`golang.org/x/text v0.43.0`
  - 待加 `modernc.org/sqlite`：纯 Go 实现，保持无 cgo 的单二进制
  - 待加 `github.com/egoist/mygo v0.7.4`：go.mod 里加 `tool github.com/egoist/mygo/cmd/mygo`，以便跑 `go tool mygo vet`（同 ccmail）
- **数据格式**：时间戳一律 unix 秒（int64）；ID 一律 base64url 无填充
- **随机源**：所有随机源以 `io.Reader` 注入，测试里喂固定字节
- **错误文本**：不出现助记词、口令、密钥、明文
- **测试写法**：每个任务先写会失败的测试；预期值写字面量，且由独立实现算出
- **测试文件只用 ASCII**：非 ASCII 字符写成 8 位的 `\U0000XXXX` 转义。原因有两个：本环境写文件时，4 位的短转义会被换成原字符；原样写入的 BOM（U+FEFF）会让 Go 编译失败
- **执行记录**：本机 `.ryugi/sdd/plan/` 下有执行记录，已被 git 忽略，不需要可以删

## 3. 剩余任务

M1 还差 Task 5–7，M2 是 Task 8–18。按顺序做，每个任务结束时测试全绿，并单独提交。

### Task 5：TOTP 与密码生成器（`vaultcrypto`）

文件：`totp.go`、`generator.go`，以及各自的测试。

**TOTP 接口**
- `type OTP struct{ Secret string; Digits, Period int; Algorithm string }`，json 标签为 `secret / digits / period / algorithm`；`ErrOTP`
- `ParseOTP(input string) (OTP, error)`
  - 接受 Base32 或 `otpauth://totp/...`
  - Base32 容忍空格、连字符、小写，有无填充均可
  - 默认值：6 位、30 秒、SHA1；`Secret` 存规范形式（大写、无填充）
  - 算法只认 SHA1 / SHA256 / SHA512，位数只认 6–8
- `TOTP(key []byte, t time.Time, period, digits int, algorithm string) (string, error)`、`(OTP).Code(t)`、`(OTP).Remaining(t) time.Duration`

**生成器接口**
- `type GenOptions struct{ Length int; Lower, Upper, Digits, Symbols, NoAmbiguous bool }`
- `DefaultGenOptions`：20 位，四类全开
- `ErrGenOptions`、`GeneratePassword(o, r)`、`(GenOptions).EntropyBits() float64`
- `StrengthLevel(bits) int`：低于 50 为弱，低于 75 为一般，低于 100 为强，其余为很强

**生成算法**（要写进向量，所以必须完全确定）
- 字符类顺序：小写、大写、数字、符号 `!@#$%^&*`
- `NoAmbiguous` 去掉 `0O1lI`
- 按上述顺序把开启的类拼成字符集，长度记为 n
- 每个字符读 1 字节 b：若 `b >= 256 - 256%n` 则丢弃重读，否则取第 `b % n` 个字符
- 生成完若缺少某个开启的类，从同一个流整串重新生成
- 长度限制 8–128；四类全关时报 `ErrGenOptions`

**RFC 6238 附录 B**（已用 Python 独立核对；8 位，30 秒）。种子：
- SHA1：ASCII 串 `12345678901234567890`（20 字节）
- SHA256：同一个串重复到 32 字节
- SHA512：同一个串重复到 64 字节

| T（秒） | SHA1 | SHA256 | SHA512 |
|---|---|---|---|
| 59 | 94287082 | 46119246 | 90693936 |
| 1111111109 | 07081804 | 68084774 | 25091201 |
| 1111111111 | 14050471 | 67062674 | 99943326 |
| 1234567890 | 89005924 | 91819424 | 93441116 |
| 2000000000 | 69279037 | 90698825 | 38618901 |
| 20000000000 | 65353130 | 77737706 | 47863826 |

**Base32 对照**
- SHA1 种子：`GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ`
- SHA256 种子：`GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZA====`
- SHA512 种子：`GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNA=`
- `JBSWY3DPEHPK3PXP` = hex `48656c6c6f21deadbeef`
- 字节 00..0f = `AAAQEAYEAUDAOCAJBIFQYDIOB4======`

**测试**
- 上表 18 个值全跑
- Base32 容错表；otpauth 链接解析（algorithm / digits / period）；非法密钥返回 `ErrOTP`
- 生成器：用手造的字节流断言字面量输出，覆盖一次丢弃字节和一次缺类重来
- 1000 个 crypto/rand 密码：每个开启的类都出现，且没有易混字符
- 强度分档的边界值

### Task 6：向量文件与 `tools/vectors`

- `spec/crypto-vectors.json` 覆盖：BIP39、密钥派生、HKDF、Argon2id、包裹、条目封装、保险库名封装、TOTP、Base32、生成器
- `go run ./tools/vectors` 生成该文件；加 `-check` 时，若生成结果与磁盘上的文件不一致，退出码为 1
- `internal/vaultcrypto/vectors_test.go` 回放文件里的每一条
- `spec/key-hierarchy.md`、`spec/item-format.md`：字节级契约（HKDF info 字符串、AAD 拼法、JSON 字段），供 M6 的 TypeScript 实现对照
- 验收：`go test ./internal/vaultcrypto/ && go run ./tools/vectors -check`

### Task 7：`internal/identity`（M1 收尾）

**keyfile**
- `Keyfile` 结构与 plan §4 的 JSON 一致
- `LoadKeyfile(path)`；`SaveKeyfile(path, kf)` 写临时文件、fsync、再 rename，权限 0600

**错误与校验**
- `ErrUnlock` 是唯一的解锁失败错误；另有 `ErrPassphraseShort`、`ErrKeyfileVersion`
- `CheckPassphrase(p)`：至少 8 个字符，按 rune 计

**账号与解锁**
- `NewAccount(mnemonic, passphrase, now, r) (*Keyfile, *Session, error)`
- `Unlock(kf, passphrase) (*Session, error)`：以下情况一律返回 `ErrUnlock`
  - Argon2 参数越界
  - 口令错误
  - 任一字段被改
  - 重新派生出的 account_id 或默认 vaultId 与文件里的不一致
- `createdAt` 不参与认证

**Session**
- 方法：`AccountID()`、`DefaultVaultID()`、`VaultKey(vaultID)`、`Lock()`（把 root 清零）、`Locked()`
- 用 `%v` 打印 Session 不能带出密钥

**测试**
- plan §10 的 M1 验收全流程，在临时目录里跑
- 每种篡改都失败
- 错误文本不含口令
- 文件权限为 0600，且不留临时文件
- 同一助记词恢复后得到同一个 account 和 vault id
- Lock 后 root 已清零

### Task 8：`internal/store`（SQLite，只存密文）

**库文件**
- 表结构照 plan §6（vaults 带 nonce / ct，保险库名加密存储）；开启 WAL
- 目录 0700，文件 0600；`-wal`、`-shm` 文件的权限也要检查
- `Home()`：默认 `os.UserConfigDir()/ccpass`，可用 `CCPASS_HOME` 覆盖
- 文件名常量：`KeyfileName = "keyfile.json"`、`DBName = "vault.db"`

**接口**
- `Open(path)`、`Close()`
- 类型：`ItemRecord`、`VaultRecord`；操作常量 `OpCreate / OpUpdate / OpDelete / OpRestore / OpPurge`；`ErrConflict`
- `PutItem(rec, op, at)`
  - items 与 oplog 在同一个事务里写
  - 新条目的 rev 必须是 1，已有条目必须是原 rev + 1，否则返回 `ErrConflict`
- `PutVault(rec)`，rev 规则同上
- 读取：`Items()`、`Vaults()`、`Oplog()`
- 键值：`Meta(key)`、`SetMeta(key, value)`

**测试**
- 权限、WAL、rev 规则
- 非法 op 让整个事务回滚
- 重开库后数据仍在
- `CCPASS_HOME` 生效

### Task 9：`internal/vault`（payload 模型 + 加密保险库）

**payload 模型**
- `Item` 字段按 plan §5；`Field{Label, Value, Kind, Secret}`；`PasswordChange{Password, ChangedAt}`
- 常量：`TypeLogin / TypeNote / TypeCard / TypeIdentity`；`SchemaVersion = 1`
- 不认识的 JSON 字段和 `type` 原样保留，重新保存时写回
- `DisplayType()` 对未知类型返回 note；`schema > 1` 时 `ReadOnly()` 为真
- `TemplateFields(typ)`（方括号标 secret 字段）
  - 信用卡：持卡人、卡号[secret]、有效期、CVV[secret]、PIN[secret]
  - 身份：姓名、电话、邮箱、地址、证件号[secret]

**保险库**
- 类型：`Entry{ID, VaultID, Rev, UpdatedAt, Deleted, Purged, Item}`、`VaultInfo{ID, Name, Rev, UpdatedAt, CreatedAt}`
- `CheckAccount(db, accountID)`：meta 里记录的 accountId 不符时返回 `ErrOtherAccount`
- `Open(db, session, now)`
  - 全部条目解密；任一条失败就返回 `*CorruptError{Count}`，其余条目也不显示
  - 首次打开时建默认保险库「个人」，id 用 `DefaultVaultID`
- 条目操作：`Create`、`Update`、`Delete`（软删除）、`Restore`、`Purge`（deleted=true + 空 payload）
- 改密码时把旧密码追加进 `passwordHistory`
- 保险库操作：`CreateVault`、`RenameVault`
- `Close()`：丢弃密钥与明文

**测试**
- 未知字段经解码再编码后仍在
- 增删改之后重开，数据仍在
- 改密文的一个字节，Open 返回 `CorruptError{Count: 1}`
- 把旧版本的行拷回去（rev 重放），保险库打不开
- 别人账号的库返回 `ErrOtherAccount`
- 改密码后历史多一条；彻底删除后是墓碑
- **db + wal 的原始字节里搜不到标题、用户名、密码、URL、保险库名**

### Task 10：搜索与筛选

- `Filter{VaultID, Type string; Trash bool}`、`Search(entries, query, f) []Entry`
- 匹配：对 title / username / urls / tags 做大小写不敏感的子串匹配；查询按空白切词，每个词都要命中
- 筛选：未知类型算作 note；Trash 只看已删除；墓碑永不显示
- 排序：先按标题（不分大小写），再按 id
- 测试：表驱动，预期的 id 写成字面量

### Task 11：界面骨架、解锁页、恢复页、`main.go`

**App 结构**
- `ccapp.Options{Home string; Rand io.Reader; Run func(work, done func()); Protect func(on bool)}`
- `New(opts)`、`(*App).View(c)`、`Lock()`、`Unlocked()`
- Argon2 在 `Run` 里异步执行，`done` 经 `Window.Update` 回到主线程

**启动页**：有 keyfile 进解锁页（只输口令）；没有则进欢迎页（新建 / 我已有助记词）

**解锁页提示**
- 失败只有一种：「口令不对，或密钥文件已损坏」
- 不足 8 位：「口令太短，至少 8 个字符」

**恢复页**
- 助记词用单行 `.Password()` 输入框；在 `OnPaste` 和内容变化时，把换行和 Tab 折成空格
- 按住「显示」才露出明文（`Element.Pressed()`）；实时显示词数
- 新口令输两次
- 错误提示：
  - 「助记词应为 24 个词，现在是 N 个」
  - 「第 N 个词不在词表里」
  - 「助记词校验和不对，请检查是否抄错」
  - 「两次输入的口令不一致」
- 本地已有属于别的账号的 vault.db 时，拒绝恢复并说明文件位置
- 成功后清空助记词和口令字段

**main.go**
- 单实例
- `Power.OnSuspend` 和 `Power.OnLockScreen` 触发 `Lock()`
- 退出前清剪贴板

**测试**（`ui.NewTester`）
- 解锁成功；错口令提示；口令太短提示
- 恢复页四种错误
- 恢复成功：写出 keyfile，并清空字段
- 拒绝恢复到别人账号的数据

### Task 12：首次创建流程

1. 生成 24 词编号展示，期间 `Protect(true)`，即 `Window.SetContentProtection` 防截屏
2. 用户点「我已抄好」
3. 随机抽 3 个不同词位复述；错了提示「第 N 个词不对」；没有跳过的路
4. 口令输两次，写出 keyfile
5. 进入主界面，`Protect(false)`

测试用固定的 `Rand`。

### Task 13：主界面（侧栏、列表、搜索、键盘）

- **侧栏**：保险库列表（切换、新建、重命名）+ 类型筛选（全部 / 登录 / 笔记 / 信用卡 / 身份）+ 已删除
- **列表**：每行显示标题、用户名、类型。用 `ui.List` + `ListState`，只建可见行，5000 条也要流畅
- **搜索与键盘**
  - 搜索框实时过滤；`Cmd/Ctrl+K` 聚焦搜索框
  - ↑↓ 移动选中，在搜索框里也有效（用 `OnTextKey`）；Enter 打开选中条目
- **出错时**：遇到 `CorruptError` 或 `ErrOtherAccount` 时显示错误，不显示数据

### Task 14：详情页

**展示**
- 密码默认显示星号，按住「显示」才露出明文
- 复制按钮：用户名、密码、验证码、自定义字段
- URL 只打开 http / https；裸域名补 `https://`；`javascript:` 等其他协议一律不打开
- TOTP：验证码 + 剩余秒数，用 `c.After` 随时间刷新
- 备注；自定义字段（secret 字段同样按住才显示）
- 密码历史：星号 + 复制按钮
- `ReadOnly()` 为真的条目显示只读提示

**测试**
- 按住之前，界面文本里没有密码
- 复制会写入剪贴板
- 打开链接用 `tt.OpenedURLs` 断言
- 固定 tester 时间下，验证码等于 RFC 的值；`Advance(30s)` 后变化

### Task 15：新建、编辑、删除、还原、彻底删除

- **新建**：按类型新建；信用卡、身份预填模板字段
- **编辑表单**
  - 字段：标题、用户名、密码、URL 列表、OTP、标签、备注、自定义字段（带 secret 开关）
  - OTP 输入用 `ParseOTP` 校验
  - 保存 / 取消
- **删除、还原、彻底删除**：删除后进「已删除」，可还原；彻底删除要二次确认
- **只读与往返**：只读条目不能编辑；编辑往返后，未知字段要保留

### Task 16：密码生成器面板

- 选项：长度 8–128、四类开关、排除易混字符
- 实时显示强度；可重新生成、复制
- 打开了条目时，可「用作当前条目密码」：更新密码，历史加一条
- 四类全关时给出提示，不生成

### Task 17：自动锁定、剪贴板、Esc、设置

**自动锁定**
- 闲置超时默认 5 分钟，可选 1–60 分钟，存在 meta 里，没有「永不」选项
- 闲置的判断：用最后一次输入时间加 `c.After`，或用 `Power.IdleTime()`
- 休眠或锁屏时立即锁定
- 锁定时丢弃：密钥、解密后的数据、编辑缓冲、生成器结果、「显示」状态
- 异步解锁的结果如果在锁定之后才回来，直接丢弃

**剪贴板**
- 复制敏感内容 30 秒后清除：先用 `ReadClipboard` 比对，内容仍是我们写入的才清
- 锁定和退出时同样先比对再清

**Esc**：先关浮层或清空搜索，没有可取消的才锁定；编辑中不响应 Esc

**测试**（用 tester 时钟）
- 闲置 5 分钟后回到解锁页；有操作则重新计时
- 改成 1 分钟后，设置持久化并生效
- 30 秒后剪贴板被清；被用户替换过的不清
- Esc 的响应顺序；编辑中 Esc 无效

### Task 18：CI、安全检查、README

- `.github/workflows/ci.yml`（参考 ccmail），依次跑：gofmt、`go vet`、`go tool mygo vet`、`go test ./...`、`go run ./tools/vectors -check`、`go build`
- 另加一步 grep：`internal/` 和 `tools/` 下出现 `math/rand`、`crypto/md5`、`crypto/des`、`crypto/rc4` 或 `cipher.NewCBC` 时 CI 失败
- README（中文）：是什么、安全模型、构建与运行、数据存放位置

## 4. 已核对的 mygo v0.7.4 API

文档在 `$(go env GOMODCACHE)/github.com/egoist/mygo@v0.7.4/docs/ui/`，建议先读 views.md、testing.md、list.md、input.md、text-input.md。

- **输入框**
  - `ui.TextInput(c, &s)`；`.Password()` 只对单行框有效，对 `TextArea` 不生效
  - 回调：`.OnPaste(func(text string) bool)`、`.OnTextKey(func(k ui.TextKey) bool)`
  - 状态：`.Changed()`、`.Submitted()`
  - 焦点：`.Focus()`、`.AutoFocus()`、`.Focused()`
- **按住**：`Element.Pressed()`
- **时间与重绘**：`c.After(d)`、`c.Now()`、`c.Invalidate()`；在其他 goroutine 里改状态用 `Window.Update(func())`
- **剪贴板与链接**：`c.WriteClipboard(s)`、`c.ReadClipboard()`、`c.OpenURL(url)`
- **快捷键**：`c.Shortcut(ui.Cmd, ui.KeyK)`；`ui.Cmd` 在 macOS 上是 Command，其他平台是 Ctrl；按键常量 `ui.KeyEnter / KeyEscape / KeyUp / KeyDown`
- **列表**：`ui.List(c, &state, n, func(i int){...})`；`ui.ListState` 有 `Key`、`Label`、`Selected` 等字段；列表本身有 `Changed()`、`Submitted()`
- **主题**：`c.Theme().Dark`、`c.Theme().TextMuted`
- **窗口与系统**
  - `Window.SetContentProtection(bool)`：防截屏
  - `mygo.Power.OnSuspend / OnLockScreen / IdleTime()`
  - `mygo.App.RequestSingleInstanceLock()`
  - `mygo.App.OnBeforeQuit / OnWillQuit / OnQuit`
  - `mygo.Shell.OpenExternal`
- **测试**
  - `ui.NewTester(view, w, h)`
  - 操作：`Click`、`Type`、`Key(mods, key)`、`Press / Release`
  - 查询：`HasText`、`Texts`、`Clipboard()`、`OpenedURLs`
  - 设置：`SetClipboard()`、`SetDark()`
  - 时钟与帧：`Advance(d)`（测试时钟，不要 sleep）、`Now()`、`Frame()`
- **不用的 API**：`mygo.Secrets` 写钥匙串时没有用户在场校验，v1 不用（见 plan §15 第 2 条）
- **生命周期检查**：`go tool mygo vet`，Element 不能存进 App 状态

## 5. 评审时重点看（测试覆盖不到的）

- 秘密进入日志、错误、panic 或 `%v` 输出（Session、Item、Keyfile、界面状态）
- 锁定后残留的密钥材料：
  - 缓存的 vault key、解密后的条目
  - 编辑缓冲、生成器结果、「显示」状态
  - 锁定之后才回来的异步解锁结果
- 异步 runner 和电源回调在非主线程上碰 App 状态
- 30 秒内退出时，剪贴板的比对清除是否生效；剪贴板管理器会记录复制过的密码
- SQLite `-wal / -shm` 文件的权限；keyfile 原子写在崩溃时的表现
- mygo 构建期生命周期（`go tool mygo vet`）
- Esc、Cmd+K 与输入框焦点、确认框之间的交互
- 5000 条时的列表虚拟化，以及每次按键的搜索开销
- 本地数据属于别的账号时的恢复和创建；keyfile 丢了但 vault.db 还在
- 未知字段和类型经过界面编辑往返后是否保留

## 6. 没做的事

- 整分支评审没跑，为了省钱。建议 M2 做完后开一个新上下文，按第 5 节评审一次
- `docs/plan.md` §16 列的 M4 之前要定的事，还没定
