# 条目格式（item format）

这份文档是 ccpass 里一条记录的字节级契约：哪些字段留在明文、密文里放什么、
什么被绑定进认证。M6 的 TypeScript 移植必须逐字节复现。

对照向量在 `spec/crypto-vectors.json` 的 `item`、`vaultMeta`、`base32` 三节；
`internal/vaultcrypto/vectors_test.go` 逐条重放。

## 1. 一条记录长什么样

表 `items` 的每一行：

| 列 | 明文还是密文 | 内容 |
| --- | --- | --- |
| `account_id` | 明文 | base64url，32 B 公钥 |
| `vault_id` | 明文 | base64url，16 B |
| `item_id` | 明文 | base64url，16 B |
| `rev` | 明文 | 整数，从 1 开始，每次修改 +1 |
| `updated_at` | 明文 | Unix 秒 |
| `deleted` | 明文 | 0 / 1 |
| `nonce` | 明文 | base64url，24 B |
| `ct` | 明文存储、密文内容 | base64url，`payload` 的 XChaCha20-Poly1305 密文 |

**标题、网址、用户名、口令、备注、分类、TOTP 种子全部在 `ct` 里。** 服务端看得见
的只有上面八列——路由和合并所需的最小集合，多一个字节都不给。

## 2. 明文字段串成 AAD

明文不是"顺便存一下"，它是被认证的数据。任何一列被改动，密文就打不开：

```
AAD = "ccpass/v1/item|" + accountID + "|" + vaultID + "|" + itemID + "|"
      + rev + "|" + updatedAt + "|" + (deleted ? "1" : "0")
```

数字用十进制，`deleted` 是 `"1"` 或 `"0"`。字段顺序固定，分隔符是 `|`。

所以下面这些事会直接失败，而不是被静默接受：

- 把一条记录搬到另一个账号、另一个库、另一个条目
- 重放一条旧 `rev`
- 改 `updated_at`
- 把 `deleted` 从 1 改回 0（或反过来）

保险库元数据（库名等）是同样的结构，AAD 少一个字段：

```
AAD = "ccpass/v1/vault-meta|" + accountID + "|" + vaultID + "|" + rev + "|" + updatedAt
```

## 3. 加密

```
nonce, ct = XChaCha20-Poly1305-Seal(key = vaultKey, msg = payload, aad = AAD, r = crypto/rand)
```

- 每个条目一把 vault key，key 来自 root key（见 `spec/key-hierarchy.md`）。
- nonce 24 字节，每次写入重新随机取。**不用计数器、不用固定 IV。**
- payload 为空就是彻底删除的墓碑：只剩 AAD 绑定着的八列明文。

## 4. payload 里放什么

payload 是 JSON，加密后整体进 `ct`。一个登录条目：

```json
{
  "title": "招商银行",
  "username": "me@example.com",
  "password": "…",
  "urls": ["https://example.com"],
  "notes": "…",
  "category": "银行",
  "otp": {
    "secret": "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ",
    "digits": 6,
    "period": 30,
    "algorithm": "SHA1"
  }
}
```

- `otp` 的四个字段与 `internal/vaultcrypto.OTP` 的 JSON 字段一一对应，缺省值
  分别是 6 位、30 秒、SHA1。
- `secret` 只存规范化的 Base32（大写、无填充）。用户粘贴进来的各种写法——小写、
  带空格、带连字符、带或不带 `=`——都在解析时归一化。
- 条目类型不同，payload 的字段就不同；`type` 之外的字段没有服务端约束，因为
  服务端看不到。

## 5. Base32 的容忍与拒绝

`ParseOTP` 接受：

- 裸 Base32 或 `otpauth://totp/...?secret=…` URI
- 小写、大写混合
- 空格、连字符、制表符、换行分隔
- 有或没有 `=` 填充

`ParseOTP` 拒绝（全部返回同一个 `ErrOTP`，错误文本不回显输入）：

- 最后一个字符带着不属于任何字节的位（非规范 Base32）
- 非 Base32 字母表的字符
- 长度超出 10–128 字节
- `otpauth` 的 URI 但类型不是 `totp`，或 scheme 不对
- `digits` 不在 6–8、`period` 不在 1–600、`algorithm` 不是 SHA1/256/512

"非规范"这一条值得强调：Go 标准库会接受 `AB`、`AC`、`AD` 三种拼法并都解成
`0x00`。打错一个字符就变成另一把能用的钥匙，所以必须做一次编码 round-trip 校验。
`spec/crypto-vectors.json` 的 `base32` 节里 `non-canonical` 就是这个用例。

## 6. 失败必须整体失败

- 打不开就是 `ErrAuth`，没有"部分解密"、没有降级显示。
- 错误的 root key、被改动的 AAD、被截断的密文，是同一条错误。
- 服务端返回的任何记录，只要 AAD 对不上，就整条丢弃并报同一个错。

## 7. 同步与冲突

- `rev` 单调递增，`updated_at` 只是给人看的。
- 两台设备同时改了同一条：两边都保留，`item_id` 不变、`rev` 各加一，让用户自己
  选留哪份。**不做 last-write-wins，不静默丢数据。**

## 8. 移植时容易错的地方

1. **AAD 的顺序和分隔符**：`|` 分隔，字段顺序固定，数字十进制，`deleted` 是
   `"1"`/`"0"` 而不是 `true`/`false`。
2. **base64url 无填充**：所有 ID、nonce、ct 都用 RFC 4648 §5 无填充 base64url。
3. **Base32 无填充**：`otp.secret` 规范化后不带 `=`。
4. **nonce 每次重新随机**：不要用计数器或时间戳，24 字节随机。
5. **`ct` 为空与 `payload` 为空是两件事**：前者是损坏，后者是墓碑。
6. **错误只有一个**：不要在错误文本里带上条目 ID、账号 ID 或失败原因。
