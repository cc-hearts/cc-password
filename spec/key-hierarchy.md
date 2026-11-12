# 密钥层级（key hierarchy）

这份文档是 ccpass 密钥树的字节级契约。M6 的 TypeScript 移植必须逐字节复现这里的
每一步；任何偏差都会让同一套助记词在不同的实现上解出不同的密钥。

所有数值都可以在 `spec/crypto-vectors.json` 的 `keyTree`、`argon2`、`wrap` 三节里
逐条对照。文中的字段名就是 JSON 里的字段名。

## 0. 名词

| 名字 | 长度 | 说明 |
| --- | --- | --- |
| seed | 64 B | BIP39 种子，`PBKDF2(mnemonic, "mnemonic" + passphrase, 2048, SHA-512)` |
| root key | 32 B | 账号的主密钥，只存在于内存 |
| identity key | 32 B 种子 / 32 B 公钥 | Ed25519，账号在网络上的身份 |
| vault key | 32 B | 单个密码库的数据密钥 |
| KEK | 32 B | 由口令用 Argon2id 导出，只用来包 root key |

root key 不落盘、不进日志、不进崩溃报告、不进 argv、不进环境变量。落盘的只有
被 KEK 包起来的 root key。

## 1. 助记词到种子

```
seed = PBKDF2-HMAC-SHA512(
    password = NFKD(mnemonic),
    salt     = "mnemonic" + NFKD(passphrase),
    c        = 2048,
    dkLen    = 64)
```

- 助记词 24 个词（BIP39，256 位熵 + 8 位校验和），先做 NFKD 归一化再喂 PBKDF2。
- 口令（passphrase，BIP39 里的 optional passphrase）可空；为空时 salt 就是
  `"mnemonic"`。
- 这一步与 BIP39 完全一致，没有自定义参数。

## 2. 种子到 root key

```
root = HKDF-SHA256(
    ikm  = seed,
    salt = SHA-256("ccpass/v1/root"),      // 32 B
    info = "root",
    L    = 32)
```

salt 是一个固定字符串的 SHA-256，不是随机值：同一套助记词在任何设备上都必须
解出同一个 root key。

## 3. root key 到身份密钥

```
identitySeed = HKDF-SHA256(root, salt = nil, info = "ccpass/v1/identity", L = 32)
identitySk   = Ed25519.NewKeyFromSeed(identitySeed)   // RFC 8032
identityPk   = identitySk.Public()
accountID    = base64url_nopad(identityPk)            // 32 B → 43 字符
```

- salt 为空时用 RFC 5869 的默认值（与哈希等长的全零字节）。
- 身份私钥不从助记词直接派生，而是从 root key 派生：这样换 root 的派生方式
  不会牵连身份。
- `accountID` 就是公钥本身，没有额外哈希。它是账号在服务端的唯一标识。

## 4. root key 到密码库

```
vaultID = base64url_nopad(HKDF-SHA256(root, nil,
             info = "ccpass/v1/vault-id|default", L = 16))
vaultKey = HKDF-SHA256(root, nil, info = "ccpass/v1/vault|" + vaultID, L = 32)
```

- 默认密码库的 ID 由 root 派生，所以每台设备恢复同一个账号时默认库的名字一样。
- 新建的库用 `NewID(crypto/rand)`，16 字节随机，base64url。
- 每个库一把独立的 vault key；库与库之间密码不通用。

## 5. 口令到 KEK，以及 root key 的包装

```
KEK = Argon2id(
    password = NFKD(passphrase),
    salt     = 16 B 随机,
    t        = 3,            // passes
    m        = 65536,        // KiB，即 64 MiB
    p        = 1,            // lanes
    dkLen    = 32)
```

默认参数就是 `DefaultArgon2`（64 MiB / 3 / 1），也是打开时允许的最弱参数：
`Check` 拒绝任何比它更弱的组合。参数不存在"快速模式"，UI 也不能调低。

```
wrapped = XChaCha20-Poly1305-Seal(
    key  = KEK,
    msg  = root,
    aad  = "ccpass/v1/wrap|" + accountID,
    nonce = 24 B 随机)
```

keyfile 保存的是 `Params`、`Salt`、`Nonce`、`CT` 四样，加上 AAD 里的
accountID。root key 本身从不落盘。

- 换口令：解出 root，用新口令重新 Argon2id + 重新 Seal。
- 口令错：`UnwrapRoot` 返回 `ErrAuth`，与其它所有认证失败同一个错误。
- accountID 错（keyfile 被搬到另一个账号）：AAD 不匹配，同样 `ErrAuth`。

## 6. 每一步的对照向量

`spec/crypto-vectors.json`：

| 节 | 覆盖 |
| --- | --- |
| `bip39` | 4 组熵值 → 助记词 → 种子（含空口令与命名口令），2 组必须被拒绝的助记词 |
| `keyTree` | 每组熵值的 root / identitySeed / identityPk / accountID / vaultID / vaultKey |
| `argon2` | 3 组口令（普通、空、含非 ASCII）的 KEK |
| `wrap` | 3 组包装：默认口令、空口令、另一个账号（后者解不开） |

`internal/vaultcrypto/vectors_test.go` 会逐条重放这些向量，`go run
./tools/vectors -check` 会在文件与代码不一致时失败。

## 7. 移植时容易错的地方

1. **NFKD**：口令和助记词都要先归一化。`é`（U+00E9）与 `e` + 组合音符必须走同
   一条路，否则中文或法文口令换台设备就打不开了。
2. **HKDF 的空 salt**：第 3、4 步的 salt 是 `nil`，按 RFC 5869 用全零，不是
   空字符串，也不是第 2 步那个 SHA-256。
3. **base64url 不带 padding**：所有 ID 和密钥的编码都是 RFC 4648 §5 的无填充
   base64url。`EncodeB64` 是唯一入口。
4. **Argon2id 不是 Argon2i/d**：混合模式，`argon2.IDKey`。
5. **Ed25519 的种子就是私钥的前 32 字节**：公钥由种子推出，不要把 64 字节的
   扩展私钥当成种子再推一遍。
