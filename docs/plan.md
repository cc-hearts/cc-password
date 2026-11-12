# ccpass 开发计划
> 自己托管、零知识的密码管理器。Go + mygo 原生控件，web3 身份模型，服务端零知识且用户可自部署。
> 本文是交接给实现模型的完整需求，可直接整份复制。
> 2026-11-12 评审后已就地修订，改了什么、为什么改见 §15；M4 之前要定的事见 §16。
> 实施进度与剩余任务见 `docs/handoff.md`。
## 1. 目标
自己用、能替代 1Password 的密码管理器：
- 桌面优先，日常查密码、生成密码、存 TOTP 要顺
- 保险库在本机加密，服务端零知识，且**用户可以自己部署那个服务端**
- 账号是助记词派生的一对密钥，没有用户名密码，不依赖任何第三方平台
- 单二进制分发，不装运行时，不连外面任何服务
**不做**：团队共享、企业策略、Web 版、云托管 SaaS、上链。
## 2. 硬约束（不可协商）
1. **零知识**：服务端永远不接触助记词、口令、根密钥、任何明文，只存密文和公钥。
2. **只用标准构件**：BIP39 / HKDF-SHA256 / Argon2id / XChaCha20-Poly1305 / Ed25519 / RFC 6238 / RFC 4648。禁止自创协议、禁止自创 KDF 组合、禁止"为了快"换弱参数。
3. **明文只允许是路由元数据**：`id / vaultId / rev / updatedAt / deleted`。标题、URL、用户名、分类、保险库名一律在密文里。
4. **认证失败必须整体失败**：错口令、密文被改、AAD 不匹配，全部报错，禁止部分解密或降级显示。
5. **不静默丢数据**：同步冲突必须留双份给用户选，禁止 last-write-wins 覆盖。
6. **数据主权**：明文导出永远可用（二次确认后）。
7. **主密钥只在内存**：不落盘、不进日志、不进崩溃报告、不进 argv、不进环境变量。
8. **不继承旧项目**：旧的 Electron/Prisma 版本只当反面教材，schema、字段、接口一概不继承。
9. **默认安全**：默认参数就是安全参数，不提供"快速模式"。
10. **中文界面优先**，文案直白，不用客服腔。
## 3. 技术栈与目录
### 桌面（主战场）
- Go 1.27.3+（mygo v0.7.4 的下限）
- [mygo](https://github.com/egoist/mygo) v0.7.x 原生控件（`github.com/egoist/mygo/ui`），锁 v0.7.4——**不嵌 webview**
- 骨架参考 `github.com/CarlOpenLab/ccmail`（Go + mygo 原生控件 + 单二进制 + 凭据加密存储）
```
cc-password/
├── main.go                  窗口入口
├── internal/
│   ├── vaultcrypto/         纯密码学，无 IO，可独立测试（含 BIP39、TOTP、生成器）
│   ├── vault/               条目 CRUD、搜索、oplog、软删除
│   ├── store/               本地库：SQLite，只存密文
│   ├── identity/            助记词生成/校验/恢复、口令包裹、解锁会话、keyfile
│   ├── sync/                ed25519 挑战认证 + 增量拉推（M4）
│   └── ccapp/               mygo ui：侧栏、列表、详情、生成器、设置
├── spec/                    规范（key-hierarchy / item-format / sync-api / crypto-vectors.json）
├── server/                  可自部署同步服务（独立 Go module，M4）
├── tools/vectors/           生成/校验 crypto-vectors.json
└── .github/workflows/       ci.yml（+ M4 的 release.yml）
```
### 手机（后置，可选）
`cc-password-native` 仓库，Expo/RN。Go 跑不进 RN，**密码学必须用 TypeScript 重写一份**，靠 `spec/crypto-vectors.json` 保证两边一致。手机端只做：解锁、查看、复制、TOTP、搜索。不做 autofill。
### 技术选择
| 项 | 选择 | 理由 |
|---|---|---|
| 本地库 | SQLite（WAL）+ 只存密文 blob | 不需要 SQLCipher，库文件泄露也只是密文 |
| UI | mygo 原生控件 | 无 webview = 无 XSS = 攻击面小一大截 |
| 免输口令 | **v1 不做** | `mygo.Secrets` 写钥匙串不带用户在场校验，缓存口令 = 口令 + keyfile 都在本机 = 主密钥实际落盘，且锁定形同虚设。M6 配合 Touch ID/Hello 门控的钥匙串条目再做 |
| 生物识别 | 后置（mygo 没内建，需 cgo 包） | M6 再评估 |
## 4. 密钥层级
```
助记词 24 词（BIP39 英文词表，256 bit 熵）
  └─ seed = PBKDF2-HMAC-SHA512(NFKD(助记词), "mnemonic"+NFKD(第25词), 2048, 64B)
       └─ root_key = HKDF-SHA256(seed, salt=SHA256("ccpass/v1/root"), info="root", 32B)
root_key（以下均为 HKDF-SHA256，salt 为空 = RFC 5869 默认的 32 个零字节）
  ├─ info="ccpass/v1/identity", 32B          → Ed25519 私钥种子 → pubkey(32B)
  │                                              └─ account_id = base64url_nopad(pubkey)
  ├─ info="ccpass/v1/vault-id|default", 16B  → 默认保险库 id = base64url_nopad(16B)
  ├─ info="ccpass/v1/vault|"+vault_id, 32B   → vault_key
  ├─ info="ccpass/v1/index|"+vault_id, 32B   → index_key（v1 不用，只保留名字）
  └─ info="ccpass/v1/device", 32B            → device_key（设备配对，M5/M6，只保留名字）
```
- `root_key` **永不变更**。换助记词 = 换账号 = 新建保险库并重加密。
- 身份密钥从 `root_key` 派生：只输口令解锁就能签同步挑战；不碰任何钱包的派生路径，同一份助记词不会和钱包撞成同一把私钥。
- 默认保险库 id 从 `root_key` 派生：新设备用助记词恢复后与老设备得到同一个 id，同步不会分叉出两个"默认保险库"。其他保险库 id 为 16B 随机。
- 助记词**不落盘、不进 keychain、不进备份**。
- 只接受 24 词、BIP39 英文词表。输入助记词要 NFKD + 小写 + 折叠空白（含换行、全角空格、NBSP），否则用户从笔记粘过来就失败。
- 第 25 词（BIP39 passphrase）内核支持，v1 界面不暴露，恒为空串。
### 口令包裹
```
kek     = Argon2id(NFKD(口令), salt=16B 随机, m=65536 KiB, t=3, p=1, L=32)
nonce   = 24B 随机
wrapped = XChaCha20-Poly1305(kek).Seal(nonce, root_key, aad="ccpass/v1/wrap|"+account_id)
```
keyfile 落盘内容（除 `wrappedRoot` 外都可公开），路径 `os.UserConfigDir()/ccpass/keyfile.json`（`CCPASS_HOME` 覆盖），权限 0600：
```json
{
  "version": 1,
  "accountId": "<base64url>",
  "vaultId": "<base64url 16B>",
  "argon2": { "mKiB": 65536, "t": 3, "p": 1, "salt": "<base64url 16B>" },
  "wrapNonce": "<base64url 24B>",
  "wrappedRoot": "<base64url>",
  "createdAt": 1762934400
}
```
Argon2 参数写在文件里 → 将来调强度不需要迁移：老文件用自己的参数解，解开后用新参数重包。口令长度下限 8（UX 策略），无上限截断。
- 读取时校验参数范围：低于默认值（m<65536、t<3、p<1）或高得离谱（m>4 GiB、t>64、p>16）一律拒绝——前者防被改弱，后者防被改成超大参数把内存撑爆。
- 解锁后用 `root_key` 重新派生 `account_id` 与默认 `vaultId`，与文件里的不一致即视为篡改。`createdAt` 只作展示，不参与认证。
- 写入走"临时文件 + fsync + rename"，不会写出半个 keyfile。
## 5. 条目格式
明文记录（落库与上行的形态，**这就是全部明文**）：
```json
{ "id": "...", "vaultId": "...", "rev": 7, "updatedAt": 1762934400,
  "deleted": false, "nonce": "<24B base64url>", "ct": "<base64url>" }
```
加密：
```
aad = "ccpass/v1/item|" + account_id + "|" + vault_id + "|" + item_id + "|" + decimal(rev)
      + "|" + decimal(updatedAt) + "|" + ("1" if deleted else "0")
ct  = XChaCha20-Poly1305(vault_key).Seal(nonce, payload_json, aad)
```
**`rev` 必须进 AAD**：旧版本密文无法重放覆盖新条目。代价是每次编辑重加密整条。
**`updatedAt`、`deleted` 也进 AAD**：明文里的每个字段都受认证，否则服务端能不动密文就把条目标成已删除。各字段都是 base64url 或十进制，不含 `|`，拼接无歧义。
payload（`ct` 解密后）：
```
schema / type / title / vaultId / createdAt / updatedAt / tags / urls /
username / password / otp{secret,digits,period,algorithm} / notes /
fields[{label,value,kind,secret}] / passwordHistory[{password,changedAt}]
```
- `type ∈ {login, note, card, identity}`；信用卡、身份的专有字段放 `fields[]`，新建时按模板预填标签
- `schema` 只在破坏性变更时 +1，新增可选字段不升版本；`schema` 高于本端支持的版本 → 只读展示
- `fields[]` 是逃生舱：1Password 有而这里没有的字段全塞进去，导入不丢信息
- 未知 `type` 降级为 `note` 展示，**不丢数据**：重新保存时保留原 `type` 和所有不认识的 JSON 字段
- 删除 = 软删除（`deleted=true`，payload 保留，可恢复）；彻底删除 = `deleted=true` 且 payload 为空串（墓碑）
搜索：**不建加密索引**。解锁后全部解密进内存，对 `title / username / urls / tags` 做大小写不敏感子串匹配；查询按空白切成多个词，每个词都要命中。5000 条以内毫秒级。
任一条记录认证失败 → 整个保险库拒绝打开并报出条目数，不跳过、不显示其余条目。
## 6. 本地存储
```sql
CREATE TABLE meta   (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE vaults (
  id TEXT PRIMARY KEY, rev INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL, nonce BLOB NOT NULL, ct BLOB NOT NULL
);
CREATE TABLE items (
  id TEXT PRIMARY KEY, vault_id TEXT NOT NULL,
  rev INTEGER NOT NULL, updated_at INTEGER NOT NULL, deleted INTEGER NOT NULL,
  nonce BLOB NOT NULL, ct BLOB NOT NULL
);
CREATE TABLE oplog (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  item_id TEXT NOT NULL, rev INTEGER NOT NULL, op TEXT NOT NULL, at INTEGER NOT NULL
);
```
- 保险库名加密：`aad = "ccpass/v1/vault-meta|" + account_id + "|" + vault_id + "|" + decimal(rev) + "|" + decimal(updatedAt)`，密钥为该保险库的 `vault_key`，payload `{"schema":1,"name":"…"}`
- 路径 `os.UserConfigDir()/ccpass/vault.db`，可用 `CCPASS_HOME` 覆盖（测试用）
- 权限 0600，目录 0700
- `items` 与 `oplog` 必须同一个事务写，否则崩溃会导致不同步
- **不持久化解锁状态**；非敏感偏好（自动锁定时长）放 `meta`
## 7. 同步协议
服务端是一个**认证过的密文 KV**，四张表：`vaults / items / challenges / tokens`。没有 users、没有邮箱、没有密码哈希。
```
GET  /v1/challenge?account=<account_id>   → nonce(32B, 一次性, 5 分钟)
POST /v1/auth  {account, nonce, sig}      → token(30 天)
GET  /v1/items?since=<cursor>&vault=<id>  → {items, cursor}
PUT  /v1/items  {items:[...]}             → {applied, conflicts}
```
- `account_id` 由服务端解出公钥，**不接受客户端另传公钥**
- 401 不区分「token 过期」与「签名错误」，避免账号枚举
- 写入必须 `rev = server_rev + 1`，否则进 `conflicts`，**不静默覆盖**
- `cursor` 用服务端自增游标，不用客户端时间戳（时钟漂移会丢更新）
- 部署：单二进制 Go + SQLite，`docker compose up -d`，TLS 由反代或内置证书终止
- 备份 = 复制那一个 `.db` 文件
- 签名消息的格式、保险库列表接口、准入控制在 M4 开工前定稿，见 §16
## 8. 界面与交互
| 区域 | 要求 |
|---|---|
| 解锁 | 日常只输口令；失败只有一种提示「口令不对，或密钥文件已损坏」；口令不足 8 位直接提示「口令太短」 |
| 恢复 | 首次在新机器上、或忘了口令时用：助记词 + 新口令。错误提示明确到「词数不对 / 第 N 个词不在词表里 / 校验和不对 / 口令太短」，提示里只给词位，不回显词 |
| 首次使用 | 生成 24 词 → 抄写 → **抽 3 个词位复述校验** → 设口令 → 落地 keyfile；跳过校验不允许；展示助记词期间窗口开防截屏 |
| 侧栏 | 保险库列表 + 类型筛选（全部/登录/笔记/信用卡/身份）+ 已删除 |
| 列表 | 标题、用户名、分类；子串搜索；上下键选择，Enter 打开 |
| 详情 | 密码默认星号可按住显示、复制按钮、URL 一键打开、TOTP 倒计时、备注、自定义字段、密码历史 |
| 生成器 | 8–128 位、四类字符开关、排除易混字符、实时强度、一键替换当前条目密码 |
| 自动锁定 | 闲置超时（默认 5 分钟，可选 1–60 分钟）/ 休眠 / 锁屏 → 立即丢弃 vault_key。**没有"永不锁定"选项** |
| 剪贴板 | 复制密码后 30 秒自动清，**先比对内容再清**（别清掉用户后复制的东西）；锁定、退出时同样先比对再清 |
| 快捷键 | `Cmd/Ctrl+K` 搜索、`↑↓` 选择、`Enter` 打开、`Esc` 先关浮层/清搜索，没有可取消的再锁定（编辑中不响应 Esc，防丢改动） |
mygo ui 要点：`Column/Row/Scroll/Text/TextInput/TextArea/Divider/Spacer/Button`，密码字段用 `.Password()`，重绘用 `c.Invalidate()`，外观从 `c.Theme().Dark` 取。
**已踩的坑**：`ui.EditableText` 是双击才变输入框，设置页里看起来像"标签下面没有输入框"——要用 `ui.TextInput`。`ui.Autocomplete` 每按一键都触发 `Changed`，直接接保存会逐键写盘。`TextArea` 调 `.Password()` 不生效（mygo 只有单行框能遮罩），所以助记词用单行遮罩框，粘贴时把换行折成空格，按住才显示。
## 9. 安全清单（逐条可验收）
- [ ] 解锁后 `root_key` 只在内存；锁定后立即丢弃引用
- [ ] 助记词字段用 `.Password()`，解锁成功后立刻清空
- [ ] 日志、panic、错误信息里不出现助记词、口令、密钥、明文密码
- [ ] `strings vault.db | grep <某个密码>` 必须搜不到
- [ ] 剪贴板 30s 自动清理，且仅在内容未被用户替换时清
- [ ] Argon2 参数不可被 UI 调低
- [ ] 所有认证失败都是同一个错误
- [ ] 密码学路径没有 `math/rand`（只用 `crypto/rand`）
- [ ] 没有 AES-CBC、没有固定 IV、没有 MD5、没有自建 KDF
## 10. 里程碑
| 阶段 | 交付 | 验收标准 |
|---|---|---|
| **M1** 密钥内核 | `vaultcrypto` + `identity` + keyfile 读写 | 新建助记词 → 设口令 → 落地 keyfile → 重启 → 用口令解开 → 同一 `account_id`；错口令、篡改 keyfile 都必须失败 |
| **M2** 本地保险库 | `store` + `vault` + 桌面 UI | 增删改查、搜索、生成器、TOTP、自动锁定、剪贴板全可用；离线连用一周不别扭；杀进程不留明文 |
| **M3** 导入导出 | 1PUX / Bitwarden JSON / Chrome CSV | 先 dry-run 报条目数与字段缺失；导入后字段抽查无误；导出能再导回来 |
| **M4** 同步 | `server/` + `sync/` | 两机交替编辑不丢数据、冲突出副本；服务端库 `strings` 搜不到任何标题 |
| **M5** 浏览器扩展 | MV3 + 本机 bridge | 登录真实网站全程不手动复制 |
| **M6** 手机 / 生物识别 | RN 复用 vectors；Touch ID/Hello（含免输口令） | 两套实现跑同一份 vectors 全绿 |
**顺序不许跳。** M2 之前不碰同步和扩展；M1 没绿不写 UI 逻辑。
## 11. 测试要求
1. **vectors 回放**：`spec/crypto-vectors.json` 每一条都跑（BIP39 / 密钥派生 / HKDF / Argon2id / 密钥包裹 / 条目封装 / 保险库名封装 / TOTP / Base32 / 生成器）
2. **外部权威向量**：RFC 6238 附录 B（`94287082` / `07081804` / `89005924` / `69279037` / `65353130`）、BIP39 官方向量（含零熵助记词）、RFC 5869、draft-irtf-cfrg-xchacha-03 附录 A.3、Argon2 参考实现向量
3. **负向用例**：错口令、密文翻一个 bit、条目被搬到别的账号 / 别的保险库 / 别的条目 / 换 `rev` 重放 / 改 `updatedAt` / 翻 `deleted`——都必须整体失败
4. **助记词拒绝**：词数不对、词表外、校验和错
5. **UI 无窗口测试**：`ui.NewTester(view, w, h)` + `Click` / `HasText`，覆盖解锁成功与失败提示、恢复页三类助记词错误
6. **CI 必须有 `vectors -check`**：防"改了实现忘了更新契约"
## 12. 用到的规范
| 用途 | 规范 | 具体参数 |
|---|---|---|
| 助记词 → 种子 | BIP39 | 256 bit / 24 词 / 英文词表；PBKDF2-HMAC-SHA512 2048 轮；salt = `"mnemonic"` + 第 25 词；NFKD |
| 派生密钥树 | RFC 5869（HKDF） | HKDF-SHA256；root 的 salt = `SHA256("ccpass/v1/root")`，子密钥 salt 为空 |
| 口令 → KEK | RFC 9106（Argon2） | Argon2id，m=65536 KiB，t=3，p=1，L=32，salt 16B |
| 条目 / 密钥封装 | draft-irtf-cfrg-xchacha-03 | 24B 随机 nonce，16B tag；底层见 RFC 8439 |
| 身份签名（登录） | RFC 8032（Ed25519） | 私钥种子 = HKDF(root_key, "ccpass/v1/identity")；对服务端下发的挑战签名 |
| 两步验证 | RFC 6238（TOTP），截断按 RFC 4226 | 默认 6 位 / 30 秒 / HMAC-SHA1；支持 SHA256/SHA512 |
| OTP 密钥编码 | RFC 4648 §6（Base32） | 容忍空格、连字符、小写、有无填充；也接受 `otpauth://totp/…` 链接 |
| ID 编码 | RFC 4648 §5（base64url 无填充） | 账号、保险库、条目 ID |
| 字符串归一化 | Unicode UAX #15（NFKD） | 助记词、口令输入前统一归一化 |
## 13. 明确不做
团队 / 共享保险库 / 权限模型、完整 Passkey 实现、审计日志、企业策略、SSO、Web 版、云托管 SaaS、计费、**区块链 / 上链**（密文上链 = 永久公开且不能删）、浏览器端存助记词或密钥。
## 14. 别踩的坑（旧项目全踩过）
| 坑 | 后果 | 正确做法 |
|---|---|---|
| 解密密钥存在服务端表里 | 能读库就能读全部密码 | 密钥只在本机，服务端拿不到 |
| 每用户一把 key + 一个固定 IV 复用 | 相同明文前缀出相同密文块，可分析、可篡改 | 每条随机 nonce + AEAD |
| MD5 存口令 | 直接爆 | Argon2id，且口令根本不发给服务端 |
| Electron `nodeIntegration:true` + `contextIsolation:false` + `webSecurity:false` | 一个 XSS 拿整库 | 不嵌 webview（mygo 原生控件） |
| 整库覆盖式同步 | 并发编辑直接丢数据 | item 级 oplog + `rev` 条件写 + 冲突副本 |
| 把标题存明文方便搜索 | 服务端知道你在哪些网站有账号 | 全加密 + 本地内存搜索 |
| 自动锁定可关 | 笔记本合盖就泄露 | 强制锁定，只调超时长度 |
| 导出藏起来 | 用户被锁死在软件里 | 导出永远可用 |
| 助记词截屏存相册 | 助记词泄露 = 全库永久泄露 | 只鼓励手抄，二次词位校验，展示时窗口防截屏 |
## 15. 修订记录（2026-11-12 评审）
| # | 改了什么 | 为什么 |
|---|---|---|
| 1 | 身份密钥改为 `HKDF(root_key, "ccpass/v1/identity")`，去掉 SLIP-0010 | 原方案 keyfile 只包 `root_key`，身份私钥却从 seed 走 SLIP-0010：只输口令解锁拿不到私钥，M4 签不了挑战；且 `m/44'/501'/0'/0'` 是 Solana 钱包默认路径，同一助记词导入 Phantom 就是同一把私钥 |
| 2 | 免输口令（`mygo.Secrets` 缓存口令）v1 不做 | mygo 写钥匙串不带用户在场校验，Linux 上同用户任何进程可读；口令 + keyfile 都在本机 = 主密钥实际落盘，强制锁定失效 |
| 3 | 条目 AAD 加入 `updatedAt`、`deleted` | 否则服务端不动密文就能把条目标成已删除，AEAD 照样通过 |
| 4 | `vaults.name` 改为密文 | 违反"明文只允许是路由元数据" |
| 5 | 默认保险库 id 从 `root_key` 派生 | 否则新设备恢复后不知道原 id，同步分叉出两个默认保险库 |
| 6 | 解锁页与恢复页分开 | 日常解锁只有口令，按"认证失败同一个错误"只能有一种提示；词数/词表/校验和三类提示属于恢复页 |
| 7 | 助记词改用单行遮罩输入框，粘贴折叠换行 | mygo 的 `TextArea` 不支持 `.Password()` |
| 8 | 子密钥 HKDF salt 为空；只支持英文词表；第 25 词界面不暴露；keyfile 路径与参数范围校验；搜索按 §5 子串匹配；Esc 先取消后锁定；锁定时长 1–60 分钟可选 | 原文未写明或前后不一，按硬约束取保守做法 |
## 16. M4 之前要定的事（不影响 M1–M3）
- **挑战签名要做域分隔并绑定服务器地址**：签 `"ccpass/v1/auth|" + server_origin + "|" + nonce`，否则恶意服务器可以把真服务器的挑战转给客户端签，拿到真服务器的 token。
- **同步 API 缺保险库列表**：新设备不知道有哪些保险库，建议改成按账号拉取（`GET /v1/items?since=` 返回该账号全部保险库的增量），保险库元数据也走同一套 rev 规则。
- **准入控制**：自部署服务器要有账号白名单（或邀请码），否则任何人都能把它当免费密文存储。
- **回滚与扣留**：`rev` 进 AAD 只防"旧密文冒充新版本"，挡不住服务端回滚或藏起条目；客户端要记住每条见过的最高 `rev` 并拒收更低的，威胁模型里写明服务端可以扣留更新。
- **keyfile 会被 Time Machine 等备份带走**：离线暴力破解只隔着一个口令；设口令页引导用 4 个以上随机词，必要时考虑把 keyfile 标记为不备份。
