# xeapi算法

- 来自: https://github.com/NeteaseCloudMusicApiEnhanced/api-enhanced/issues/174#issuecomment-4565170281
- 作者: 1254qwer

以下分析结果我用的GPT-5.5对Android端做的逆向，在本地抓过几个包做验证，仅供参考，尽管我在本地有一定验证，但仍然可能出错，希望能有些帮助

<details>
<summary>使用Gemini整理的请求方式，请注意 AI 很可能会出错</summary>

# 网易云音乐 xeapi (AegisSDK) 协议实现指南

`xeapi` 是网易云音乐的新一代 API 加密包装层（AegisSDK）。它通过非对称加密（X25519）、动态密钥和复杂的中间变换，将原本的 `/api/` 或 `/eapi/` 请求封装为更安全的格式。

最终的 HTTP POST 请求体由三个核心参数构成：`B`（业务数据）、`S`（动态密钥信封）和 `R`（版本与会话信息）。

## 1. 全局常量与加密基础

在开始实现之前，需要在代码中固化以下全局常量。这些常量用于基础的签名和静态解密。

* **静态 AES 密钥 (Static Key)**: `ab1d5a430f6bb04a3f01e81ddd72bd916d5ce591248ac128714806d7f8fb1b84` (Hex 格式，32 字节)
* **请求签名密钥 (Sign Key)**: `mUHCwVNWJbunMqAHf5MImuirT6plvs6VSFW62MGHstFQxhBGdEoIhLItH3djc4+FB/OKty3+lL2rGeoFBpVe5g==` (Base64 格式)
* **百分号编码规则**: 仅保留字母、数字和 `-._~`，其余字节均编码为大写的 `%XX`。

## 2. 公钥获取与更新 (Key Refresh)

`xeapi` 的加密依赖服务端的公钥。客户端必须在本地没有有效缓存时，主动向服务端请求公钥信息。

**请求端点**: `POST /eapi/gorilla/anti/crawler/security/key/get` (使用传统的 eapi 包装格式)

**请求明文 JSON 结构**:
你需要生成以下字段，并将其与设备运行时的反垃圾 Token（如 `t1`, `t2`, `deviceId`, `uid`）合并发送。

```json
{
  "currentKeyVersion": "", 
  "timestamp": "1779955023124", 
  "nonce": "1234567890123456", 
  "requestType": "active",
  "signature": "...",
  "os": "android",
  "appVersion": "9.5.15",
  "deviceId": "...",
  "uid": "..."
}

```

* **nonce**: 16 位十进制随机数字符串。
* **signature**: `Base64(HMAC-SHA256(Sign_Key, timestamp + nonce))`。

**响应解析**:

1. 解密外层的 eapi 响应体。
2. 提取 `data.encryptedData`、`data.signature` 和 `data.timestamp`。
3. **验签**: 验证 `data.signature` 是否等于 `Base64(HMAC-SHA256(Sign_Key, data.timestamp + 请求时的nonce))`。
4. **解密**: 使用 **静态 AES 密钥** (AES-256-ECB + PKCS#7) 解密 `encryptedData`，再进行 Base64 解码，得到最终的公钥 JSON 缓存：

```json
{
  "publicKey": "3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=",
  "version": "1000000000000",
  "nextUpdateTime": 1803882269000,
  "sk": "8PZfbIFA1779944463972"
}

```

## 3. 构建 xeapi 请求

当需要发送业务请求时，将原 URL 的 `/api/` 或 `/eapi/` 替换为 `/xeapi/`，并按以下步骤生成 `B`、`S`、`R` 参数。

### Step 3.1: 准备明文信封 (Plaintext Envelope)

将原始请求信息打包成一个 JSON 字符串。格式如下：

```json
{
  "method": "GET", 
  "contentType": "application/json", 
  "queryString": "id=123&e_r=true", 
  "body": "Base64(原始Body内容, NO_WRAP)"
}

```

*注：只在非 POST 时添加 `method`，非表单时添加 `contentType`。必须向 `queryString` 中注入 `e_r=true`。*

### Step 3.2: 确定动态密钥 (Dynamic Key)

* **有可用会话**: 如果之前的响应头返回了 `x-encr-ssid` 和 `x-encr-sskey`，则直接将 `x-encr-sskey` 的 **ASCII 字符串字节**（32 字节）作为动态密钥，`sessionId` 设为 `x-encr-ssid` 的值。
* **无可用会话**: 生成 16 字节的随机字节作为动态密钥，`sessionId` 设为空字符串 `""`。

### Step 3.3: 生成 R 参数 (版本与会话)

* **明文**: `公钥的version + "|" + sessionId` (例如：`1000000000000|01c3a3532...`)
* **加密**: 使用 **静态 AES 密钥** (AES-256-ECB + PKCS#7) 加密。
* **输出**: `UrlEncode(Base64(密文))`

### Step 3.4: 生成 B 参数 (业务数据)

此参数经历两层 AES 加密和一层中间混淆变换。

1. **内层加密**: 使用 **静态 AES 密钥** (AES-256-ECB) 对 Step 3.1 生成的明文 JSON 进行加密。
2. **中间变换 (Transform)**:
* 生成 16 字节随机数 `r`。
* 将内层密文的每一个字节与 `r[i % 16]` 进行 XOR 异或。
* 对异或后的数据进行 Base64 编码。
* 计算位移量：`rot = (r[0] & 0x0F) % 编码后数据的长度`。
* 将 Base64 字符串向左循环移位 `rot` 个字节。
* 拼接结果：`最终变换数据 = r 的字节流 + 移位后的 Base64 字节流`。


3. **外层加密**: 使用 **动态密钥 (Dynamic Key)** (AES-256-ECB 或 AES-128-ECB，取决于密钥长度) 对上述变换数据进行加密。
4. **输出**: `UrlEncode(Base64(密文))`

### Step 3.5: 生成 S 参数 (密钥信封)

如果使用的是复用的会话密钥（即存在 sessionId），由于服务端已知道密钥，这一步的计算可以与新生成密钥时保持一致，或根据具体实现简化。标准的 S 参数生成逻辑如下：

1. **明文**: `Base64(动态密钥) + "|android|" + 公钥的sk`
2. **密钥交换 (X25519)**:
* Base64 解码服务端的 `publicKey` (32 字节)。
* 本地生成临时的 X25519 密钥对，导出临时公钥 (Ephemeral Public Key, 32 字节)。
* 使用本地私钥和服务端公钥执行 ECDH，计算出共享密钥 (Shared Secret)。


3. **密钥派生 (HKDF-SHA256)**:
* `PRK = HMAC-SHA256(key=32字节的0x00, data=Shared Secret)`
* `AES-GCM Key = HMAC-SHA256(key=PRK, data=临时公钥 + 0x01 的单字节)` 取前 16 字节。


4. **加密 (AES-GCM)**:
* 生成 12 字节的随机 IV。
* 使用派生出的 AES-GCM Key 对明文进行加密，提取 16 字节的 Tag。


5. **拼接封装**: `临时公钥 (32B) + 随机 IV (12B) + AES-GCM 密文 + GCM Tag (16B)`
6. **输出**: `UrlEncode(Base64(拼接结果))`

### Step 3.6: 发送请求

构建最终的 HTTP Body，并添加必要的 Header：

```http
POST /xeapi/path/to/api
X-Client-Enc-State: ENCRYPTED
Content-Type: application/x-www-form-urlencoded

B=...&S=...&R=...

```

## 4. 响应解析与会话保持

1. **提取会话参数**: 检查 HTTP 响应头，如果包含 `x-encr-ssid` 和 `x-encr-sskey`，请将其保存下来，供下一次请求作为动态密钥使用（实现 Session 复用，减少 X25519 计算开销）。
2. **解密响应体**: 目前观察到，`xeapi` 接口的响应体 **仍然使用传统的 eapi 响应加密格式**。
* 即：使用老旧的密钥 `e82ckenh8dichen8` 进行 AES-128-ECB 解密。
* 如果解密后的明文包含 GZIP 魔法头，则进行解压，最终得到业务响应 JSON。

</details>

<details>
<summary>GPT5.5的逆向笔记，基于Android 9.5.15版本，native使用arm64-v8a，请注意 AI 很可能会出错</summary>

# xeapi / AegisSDK research notes

## Java request wrapper

- `sources/n72/a.java` builds a JSON envelope from the original request and calls `IEncryptService.encrypt(jsonString)`.
- The URL is rewritten from `/api/` to `/xeapi/`.
- Final request body is the native encrypted string.

`n72.a.h(Request)` builds the xeapi plaintext JSON from the original request:

- URL for the envelope is first normalized by replacing `/eapi/` with `/api/`.
- Final network URL is then made by replacing `/api/` with `/xeapi/`.
- If the original body exists, the complete raw body bytes are Base64-encoded
  with Android `Base64.NO_WRAP` and stored as `"body"`.
- If the original method is not `POST`, it stores `"method"`.
- If the body content type exists and the media type is not
  `application/x-www-form-urlencoded`, it stores `"contentType"` as the full
  media type string.
- If the normalized URL has an encoded query, it stores `"queryString"`.
- `appendErKey(..., xeapi=true)` then appends the encryption-report key into
  `"queryString"`. For normal `v62.a` requests this key is `"e_r"`, unless the
  request already supplied it.

The resulting `JSONObject.toString()` is the exact plaintext passed into native
`AegisNative.encrypt`. Because this is a normal FastJSON `JSONObject`
construction in decompiled code, do not assume a stable pretty format; compare
against captured plaintext/log output when exact bytes matter.

## Java to native

- `sources/com/aegis/sdk/AegisNative.java` loads `libAegisSDK.so`.
- `sources/p62/c.java` calls:
  - `AegisNative.initializeEngine(public_key_path, staticKey, deviceId, "android", ua, signKey, networkLayer, config)`
  - `AegisNative.encrypt(data)`
  - `AegisNative.setSession(sessionId, sessionKey)`
- Java obfuscated init strings decoded by `NeteaseMusicUtils.decodeCache` use
  a single-byte XOR key `0xa3`, confirmed by Frida hooks on
  `NeteaseMusicUtils.p/q` and `nu.a`.
  - static key blob: `qx1aQw9rsEo/Aegd3XK9kW1c5ZEkisEocUgG1/j7G4Q=`
  - sign key: `mUHCwVNWJbunMqAHf5MImuirT6plvs6VSFW62MGHstFQxhBGdEoIhLItH3djc4+FB/OKty3+lL2rGeoFBpVe5g==`
  - The previously assumed repeating key `[0xa1, 0xa3, 0xa2, 0xa3]` was wrong.

## Native encrypt entry

- `Aegis_Encrypt` -> `sub_116F58`.
- Output format:
  - `B=<urlencode(base64(B_raw))>&S=<urlencode(base64(S_raw))>&R=<urlencode(base64(R_raw))>`
- `sub_12FF0C` is percent encoding:
  - leaves alnum and `-._~`
  - percent-encodes other bytes as uppercase `%XX`

## Engine state offsets

- `engine + 8`: ECC cipher object
- `engine + 16`: dynamic/session AES cipher object
- `engine + 24`: static AES cipher object
- `engine + 80`: decoded static key
- `engine + 128`: local dynamic key
- `engine + 152`: dynamic key creation time
- `engine + 160`: dynamic key interval
- `engine + 168/192/216/240`: runtime info strings from init: `os`, `deviceId`, `ua`, `signKey`
- `engine + 264/288`: session id / session key
- `engine + 328`: mutex

## Public key state

Stored by `sub_12B188`:

- offset `0`: `publicKey`
- offset `24`: `version`
- offset `48`: `nextUpdateTime`
- offset `56`: `sk`

Loaded from local public key JSON fields:

- `publicKey`
- `version`
- `sk`
- `nextUpdateTime`

App-side cache path:

```text
ApplicationWrapper.getInstance().getFilesDir()/aegissdk/public_key
```

The file content is Base64 text. Native Base64-decodes it, parses JSON, and
loads the four fields above. During initialization it logs `Always updating
public key on initialization.` and calls `sub_1164C8(..., active=true)` even
after reading a local cache.

## Public key update protocol

Java side `p62/e.java` handles native `requireKey(type=0, data, callbackHandle)`:

- parses native `data`
- adds `t1 = static key blob`
- adds `t2 = sign key`
- adds `os = android`
- adds `appVersion`, `deviceId`, and `uid`
- sends request to `gorilla/anti/crawler/security/key/get`
- passes the response string back through `AegisNative.onNetworkResponse`

`sub_12D504` is the native-to-Java bridge for this. It looks up
`requireKey(int, String, long)` with JNI signature `(ILjava/lang/String;J)V`
and calls it with:

- `type = 0`
- `data = compact native JSON`
- `callbackHandle = native callback object pointer`

`v62.a("gorilla/anti/crawler/security/key/get", object.getInnerMap()).E0(true).s()`
creates a normal `POST` request. The body before OkHttp interceptors is an
`application/x-www-form-urlencoded` `FormBody` containing the native JSON fields
plus Java-added `t1/t2/os/appVersion/deviceId/uid`. `E0(true)` sets
`f.a0() == true`, which the general encrypt interceptor treats as a bypass for
automatic xeapi upgrade unless the request is explicitly forced with `G0(true)`.
Therefore current evidence says the key update request is not itself wrapped by
xeapi at the Java/native boundary; any additional transformation would need to
come from other global interceptors or lower network layers.

`sub_1164C8` builds the native request payload with:

- `currentKeyVersion`
- `signature`
- `timestamp`
- `nonce`
- `requestType`: `active` or `passive`

`sub_12B8D4` generates the request timestamp/nonce/signature tuple:

- `timestamp`: `std::chrono::system_clock::now() / 1000` converted to decimal
  string.
- `nonce`: 16 iterations of `uniform_int_distribution<int>(0, 9)` backed by an
  MT19937-style PRNG seeded from `/dev/urandom`; each digit is converted with
  `std::to_string` and appended. The final nonce is exactly 16 decimal digits.
- `signature`: `base64(HMAC-SHA256(signKey, timestamp + nonce))`.

`sub_12BD00(timestamp, nonce)` confirms the concatenation order: timestamp
first, nonce second.

The native request JSON is serialized compactly, equivalent to:

```json
{"currentKeyVersion":"...","signature":"...","timestamp":"...","nonce":"...","requestType":"active"}
```

`requestType` is `"active"` for initialization/explicit update and `"passive"`
for passive refresh.

`sub_128D70` handles the response. For status code 200 it expects:

```text
data.encryptedData
data.signature
data.timestamp
```

It verifies:

```text
expected = base64(HMAC-SHA256(signKey, str(responseTimestamp) + requestNonce))
```

`requestNonce` is saved in the native network callback object created by
`sub_1164C8`: the object layout starts with vtable, then engine/context pointer,
then the nonce string at offset `+16`. `sub_128D70` reads that saved nonce when
verifying the response.

Only after signature verification passes does it call `sub_119440` to decrypt
and parse the new public key data. `sub_119440` Base64-encodes the decrypted
response and writes it to the local public key file, then parses the JSON fields
and stores them in native state.

`sub_116C88` performs the decrypt step:

1. Base64-decode `encryptedData`.
2. Decrypt with the static AES object (`engine + 24`), i.e. AES-256-ECB +
   PKCS#7 using the decoded static key.

With the Frida-confirmed static key, the captured `encryptedData` decrypts to:

```json
{"publicKey":"3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=","version":"1000000000000","nextUpdateTime":1803882269000,"sk":"8PZfbIFA1779944463972"}
```

## AES modes

`sub_12D8F4` stores mode at cipher object `+56`.
`sub_12DA54` maps mode/key length to OpenSSL EVP cipher structures.

- mode `0`: AES-CBC, IV length 16
- mode `1`: AES-ECB, IV length 0
- mode `2`: AES-GCM, IV length 12, tag 16

Initialization:

- `engine + 8` ECC cipher contains an internal AES-GCM object, mode `2`
- `engine + 16` dynamic/session AES cipher is mode `1`
- `engine + 24` static AES cipher is mode `1`

`sub_12DBBC` uses OpenSSL EVP encrypt calls. For ECB/CBC it does not disable
padding, so the AES-ECB layers use OpenSSL default PKCS#7 padding. For GCM it
sets/uses a 12-byte IV and extracts a 16-byte tag.

## B / business data

`sub_118538`:

1. AES-ECB with static key over original JSON.
2. `sub_12F4C4` transforms the first ciphertext.
3. AES-ECB with dynamic/session key over transformed data.

The raw ciphertext is Base64-encoded by `sub_116F58` before being placed in the
`B=` form value.

The static key blob decodes directly to 32 bytes:

```text
ab1d5a430f6bb04a3f01e81ddd72bd916d5ce591248ac128714806d7f8fb1b84
```

So this layer is AES-256-ECB + PKCS#7.

`sub_12F4C4` exact transform:

1. Generate 16 random bytes `r` with `sub_211430`.
2. Copy first AES ciphertext to a mutable string.
3. XOR every ciphertext byte with `r[i & 0xf]`.
4. Base64-encode the XORed bytes via `sub_12F1A8`.
5. Let `rot = (r[0] & 0xf) % len(base64)`.
6. Rotate the Base64 string left by `rot` bytes.
7. Output:

```text
r || rotated_base64(xor(ciphertext, r))
```

## S / dynamic key envelope

`sub_118738` builds plaintext:

```text
base64(dynamicKey) + "|" + os + "|" + sk
```

Then `sub_12A968` performs:

1. Base64-decode peer public key; decoded length must be 32 bytes.
2. X25519/ECDH.
3. HKDF-like expansion derives 16-byte AES-GCM key.
4. Generate 12-byte random IV.
5. AES-GCM encrypt the plaintext.
6. Output bytes:

```text
ephemeralPublicKey || iv || ciphertext || gcmTag
```

The raw envelope is Base64-encoded by `sub_116F58` before being placed in the
`S=` form value.

`sub_12FAF8` is HMAC-SHA256:

- It calls OpenSSL HMAC implementation (`crypto/hmac/hmac.c`).
- Digest object returned by `sub_1CA724` has output size `0x20` and block size `0x40`, matching SHA-256.

`sub_12FBE0` is the HKDF-like function used by `sub_12A968`.
The call from `sub_12A968` is:

```text
sub_12FBE0(shared_secret, zero32_string, ephemeral_public_key, 16)
```

`xmmword_A04C0` initializes the libc++ short string header for a 32-byte string;
the heap buffer is zeroed, so the salt is 32 zero bytes.

`sub_12FAF8(key, data)` calls HMAC-SHA256. Therefore the derivation is:

```text
PRK = HMAC-SHA256(zero32, shared_secret)
T(1) = HMAC-SHA256(PRK, ephemeral_public_key || 0x01)
AES-GCM key = first 16 bytes of T(1)
```

For longer output the loop is the normal HKDF expand shape:

```text
T(n) = HMAC-SHA256(PRK, T(n-1) || info || n)
```

X25519 virtual call details:

- `sub_12A968` invokes ECC vtable entry at `off_2CB3C0 + 0x10`, address `0x12ee10`, an internal entry inside `sub_12E714`.
- The X25519 branch:
  - validates peer key length is 32 bytes.
  - loads peer key using EVP_PKEY raw-key style OpenSSL calls.
  - generates an ephemeral X25519 key.
  - writes 32-byte raw ephemeral public key to the third output argument used later as the first bytes of `S`.
  - derives shared secret and writes it to the second output argument used as input to `sub_12FBE0`.

## R / version info

`sub_118BF0`:

- Requires public key `version`.
- Plaintext:

```text
version + "|" + sessionId
```

- Encrypted with static AES-ECB.
- The raw ciphertext is Base64-encoded by `sub_116F58` before being placed in
  the `R=` form value.

## Dynamic/session key selection

`sub_116F58` first checks session state loaded by `sub_12B858`:

- `engine + 264`: session id
- `engine + 288`: session key

If both are non-empty, it logs `Using session key as dynamic key` and uses the
session key for the B second AES layer and for the S plaintext. Otherwise it uses
the local dynamic key at `engine + 128`, generating a fresh 16-byte key when it
is missing or expired.

`Aegis_SetSession` -> `sub_11A160` -> `sub_12B694` stores these fields.

## Dynamic key generation

`sub_12BE10` generates key bytes.

- Initialization asks for 128 bits, resulting in 16 bytes.
- Primary random path uses OpenSSL RAND-style `sub_211430`.
- Fallback path uses `/dev/urandom` / `std::random_device` and MT-style state.
- `Aegis_InitializeEngine` receives Java's
  `AegisEncryptConfig.getAegisUpdateIntervalMinute()` as its final integer
  argument. Java clamps this to at least `1`.
- Native stores the dynamic key creation time as `system_clock::now() / 1000`
  and the interval at `engine + 160`.
- `sub_116F58` checks local dynamic-key expiry as:

```text
expired = (now_seconds - created_seconds) >= 60000 * interval
```

So with the default interval `1`, the native local dynamic key expires after
`60000` seconds. This looks like a unit mismatch with the Java property name
`aegisUpdateIntervalMinute`, but the native comparison above is direct.

Java also has a separate public-key update throttle:

```text
AegisEncryptConfig.publicKeyUpdateIntervalSecond = 120
```

`p62/e.java` uses this in `d()` before calling `AegisNative.updatePublicKey`.

## Next targets

- Find real xeapi samples/logs to validate the Python reproduction end to end.
- Locate public key cache/config on disk and decode a real `publicKey/version/sk`.
- Inspect the public-key update request encryption/decryption helpers around
  `sub_116C88`, `sub_12C7C8`, and `sub_129D2C`.

## Reproduction helper

`tools/xeapi_crypto.py` implements the confirmed pieces:

- static key decode
- approximate Java xeapi plaintext JSON envelope builder
- AES-ECB + PKCS#7
- `sub_12F4C4` B middle transform
- Aegis percent encoding
- `S` X25519/HKDF-SHA256/AES-GCM envelope skeleton
- `R = AES-ECB(staticKey, version + "|" + sessionId)`
- public key update response signature helper
- public key update request signature helper
- native-shaped 16-digit public key request nonce generator
- public key update native request JSON builder
- public key update `encryptedData` decrypt helper

## Public-key update request, Java side parameters

Important naming correction: the constants in `p62/f.java` are native
initialization inputs, not the HTTP `t1` / `t2` fields for
`gorilla/anti/crawler/security/key/get`.

Native init still receives:

- static key blob from `p62.f.h()`:
  `qx1aQw9rsEo/Aegd3XK9kW1c5ZEkisEocUgG1/j7G4Q=`
- native init sign key from `p62.f.g()`:
  `mUHCwVNWJbunMqAHf5MImuirT6plvs6VSFW62MGHstFQxhBGdEoIhLItH3djc4+FB/OKty3+lL2rGeoFBpVe5g==`

But `p62/e.java` builds the public-key update request like this:

```java
object.put("t1", this$0.h());
object.put("t2", this$0.g());
object.put("os", "android");
object.put("appVersion", this$0.e());
object.put("deviceId", k4.e());
object.put("uid", this$0.f());
new v62.a("gorilla/anti/crawler/security/key/get", object.getInnerMap()).E0(true).s()
```

The concrete implementation is `p62/g.java`:

- `t1 = k21.b.F0()`
- `t2 = s72.b.f288632a.m()`
- `appVersion = f0.b(ApplicationWrapper.getInstance())` in release builds
- `uid = sn.a.m().u()`

### t1 / checkToken

`k21.b.F0()` is the app-wide `securityGetToken` bridge:

```java
return (String) E("securityGetToken").c();
```

The login bridge maps `securityGetToken` to `mj0.f.f241775b1`, which calls
`mj0.e.d1()`, which returns `rj0.c.b()`.

`rj0.c.b()`:

- checks that the YD/security environment is available.
- gets `jj.a` from `ServiceFacade`.
- calls `initShield("YD00000558929251")`.
- calls `getToken()`.

`com.netease.cloudmusic.core.security.Security.getToken()` uses
`WatchMan.getToken(500, callback)` if Shield has been initialized.

So HTTP `t1` is the same dynamic WatchMan/Yidun security token used as
`checkToken` on many normal business APIs. It is not the static Aegis key blob
and is not realistically derivable offline from the Aegis native code alone.

### t2 / YD device token

`s72.b.m()` returns an 易盾 device fingerprint token:

```java
NEDevice.get().getToken("946be734f7a741f5b1f36970b3075c7f")
```

The successful token is cached in process as `cachedYdToken`. The same manager
can also call:

```text
middle/device-info/get?ydDeviceType=Android&ydDeviceToken=<token>
```

and store returned `sDeviceId` cookies / server device-id corrections.

Therefore HTTP `t2` is also a runtime SDK token, not a fixed app constant.

### appVersion

Release `appVersion` is `PackageInfo.versionName`:

```java
f0.b(context) -> rt.b(context) -> packageInfo.versionName
```

Debug builds can override it with `cn.h()`.

### deviceId

`deviceId = k4.e()`.

Priority:

1. in-memory cached `f124759a`
2. non-main process cached `cachedDeviceId`
3. persisted `serverDeviceId`
4. persisted `encrypt_deviceId`
5. legacy plain `deviceId`
6. generated local id

The generated local format is:

```text
urlencode(base64(imei + TAB + wifi + TAB + android_id + TAB + local_id_slice))
```

`local_id_slice` is from `NEDeviceID.getLocalID(context)`:

- empty -> `"null"`
- length > 24 -> substring `[8:24]`
- length > 20 -> substring `[0:20]`
- otherwise whole trimmed value

`k4.I(context, deviceId)` validates by URL-decoding, Base64-decoding, splitting
on TAB, and checking field 3 (`android_id`) against the current device. A server
returned `serverDeviceId` / `sDeviceId` can replace this local generated form.

### key/get path and wrapping

The public-key update path in Java is relative:

```text
gorilla/anti/crawler/security/key/get
```

`v62.a.z()` passes relative paths to:

```java
m62.a.k(https, true, str)
```

and `m62.a.k()` builds:

```text
scheme + "://" + apiDomain + "/eapi/" + path
```

because the second boolean is `true`.

`p62/e.java` additionally calls `E0(true)`, and `n72.a.intercept()` reads
`fVar.a0()` into `z16`. This prevents automatic xeapi wrapping for this request:

```java
z15 = sdkInitialized && (force || (enabled && ... && !z16 && !blacklist))
```

Since `z16` is true, `key/get` falls through to normal eapi wrapping. The
non-xeapi branch serializes the JSON with `NeteaseMusicUtils.serialdata(url,
plainParams)` and posts a form body:

```text
params=<serialdata output>
```

So the expected HTTP request is a standard eapi POST to:

```text
https://<api-domain>/eapi/gorilla/anti/crawler/security/key/get
```

where the plaintext JSON includes the native generated fields
`currentKeyVersion`, `signature`, `timestamp`, `nonce`, `requestType`, plus the
Java-added fields `t1`, `t2`, `os`, `appVersion`, `deviceId`, and `uid`.

Captured 9.5.15 traffic confirms this endpoint and eapi wrapping:

```text
POST /eapi/gorilla/anti/crawler/security/key/get
Host: interface3.music.163.com
Content-Type: application/x-www-form-urlencoded
```

The decrypted eapi plaintext shape was:

```text
/api/gorilla/anti/crawler/security/key/get
-36cd479b6b5-
{...json...}
-36cd479b6b5-
md5
```

Notable JSON fields from capture:

- `appVersion`: `"9.5.15"`
- `currentKeyVersion`: `""` on first/no-cache update
- `deviceId`: captured `k4.e()` value
- `e_r`: `true`
- `header`: `"{}"`
- `nonce`: 16 decimal digits
- `os`: `"android"`
- `requestType`: `"active"`
- `signature`: native request signature
- `timestamp`: decimal milliseconds string
- `uid`: string user id
- `t1`: WatchMan/YD token from `k21.b.F0()`
- `t2`: NEDevice token from `s72.b.m()`
- `checkToken`: also present

`checkToken` was not inserted by `p62/e.java` itself. It is added by general
network anti-spam logic:

- `v62.a.U0()` calls `AntiSpamService.appendMusicYdTokenWithUrl(url, map)`.
- `MusicAntiSpamManager.appendYdToken()` adds `checkToken = k21.b.F0()` if the
  URL is in the anti-spam config.
- `network/interceptor/c.java` separately adds header `X-antiCheatToken` with
  `v52.t.getToken()` if needed.

In the captured request, `checkToken`, `t1`, and `X-antiCheatToken` were all
WatchMan-like tokens, but not byte-identical. Treat them as independently
obtained runtime tokens, even if they are generated through the same service.

The captured response after eapi decrypt + gzip decompress was:

```json
{"code":200,"data":{"encryptedData":"...","signature":"...","timestamp":1779955023124,"code":200,"message":"success"},"message":""}
```

This mismatch was later resolved. The bad constants above came from using the
wrong `NeteaseMusicUtils.decodeCache` XOR key. Runtime Frida hooks on
`nu.a()` and `NeteaseMusicUtils.p/q()` show the decode operation is a
single-byte XOR `0xa3`.

The corrected 9.5.15 Aegis constants are:

- sign key:
  `mUHCwVNWJbunMqAHf5MImuirT6plvs6VSFW62MGHstFQxhBGdEoIhLItH3djc4+FB/OKty3+lL2rGeoFBpVe5g==`
- static key blob:
  `qx1aQw9rsEo/Aegd3XK9kW1c5ZEkisEocUgG1/j7G4Q=`
- decoded static AES key:
  `ab1d5a430f6bb04a3f01e81ddd72bd916d5ce591248ac128714806d7f8fb1b84`

Native argument mapping was rechecked:

- Java `initializeEngine(path, staticKey, deviceId, os, ua, signKey, networkLayer, config)`
  is converted to C strings in the same order.
- `Aegis_InitializeEngine(..., a6=signKey, ...)` passes `a6` into
  `sub_115010`.
- `sub_115010` stores that value with `sub_12B45C`; `sub_1164C8` later copies it
  with `sub_12B4B4` and calls `sub_12B8D4`.
- `sub_12FAF8(a, b)` is `HMAC-SHA256(key=a, data=b)` and `sub_12F1A8` is normal
  Base64.

With the corrected constants:

- public-key request `signature` is
  `base64(HMAC-SHA256(signKey, timestamp + nonce))`.
- public-key response `signature` is
  `base64(HMAC-SHA256(signKey, responseTimestamp + requestNonce))`.
- captured `data.encryptedData` decrypts with the static AES key to the public
  key JSON shown above.

## Header observations from rooted-device capture

Do not store login cookies/tokens in this note. Only format/source facts:

- `User-Agent` format observed:
  `NeteaseMusic/<versionName>.<buildver>(<versionCode>);<system http.agent>`.
  Code source is `v52/b.java r()`:
  `NeteaseMusic/` + `NeteaseMusicUtils.J(context)` + `(` +
  `NeteaseMusicUtils.H(context)` + `)` + `;` + `System.getProperty("http.agent")`.
- `x-appver` is `CloudMusicCookieStore.getInstance().getAppVer()`, matching
  cookie `appver` / package `versionName` such as `9.5.05`.
- `x-buildver` is `BuildInfo.f56675a`, matching cookie `buildver` such as
  `260427110037`.
- `x-deviceId` is `k4.e()`.
- `x-sDeviceId` comes from cookie `sDeviceId`.
- `x-os` comes from cookie/store OS (`android`).
- `x-osver` is `CloudMusicCookieStore.validateAndEncode(NeteaseMusicUtils.q0())`,
  matching Android release such as `16`.
- `x-music-u` mirrors login cookie `MUSIC_U`; it is a bearer credential and not
  a stable algorithmic parameter.

The captured deviceId sample decodes as:

```text
null<TAB>02:00:00:00:00:00<TAB>android_id<TAB>local_id_slice
```

This matches `k4.e()` local generation exactly: missing IMEI, default Android
Wi-Fi MAC, current Android ID, and sliced `NEDeviceID.getLocalID()`.

`x-aeapi: true` appears in two places:

- `n80/a.java`: ad/biz interceptor adds it when `enableGzip` is true.
- `com/netease/cloudmusic/network/interceptor/k.java`: network optimization
  interceptor adds it for matched requests / AB configuration.

This header is not the Aegis `xeapi` body format. It can appear on normal eapi
requests such as `/eapi/user/safe/bindings/...`.

## Practical parameter collection plan

Stable or directly readable from a normal rooted/captured app session:

- `deviceId`: use `x-deviceid`, `deviceId` cookie, or `k4.e()` output.
- `sDeviceId`: use `x-sdeviceid` or `sDeviceId` cookie when present.
- `appVersion`: `x-appver` / cookie `appver` / package `versionName`.
- `buildver`: `x-buildver` / cookie `buildver` / `BuildInfo.f56675a`.
- `uid`: current login user id, also visible in many app/profile stores.
- login cookie: `MUSIC_U` / `x-music-u`, required for authenticated requests but
  should be treated as a secret.

Runtime SDK tokens that likely need in-app execution/hooking:

- `t1`: `k21.b.F0()` -> `securityGetToken` -> `rj0.c.b()` ->
  WatchMan Shield token with id `YD00000558929251`.
- `t2`: `s72.b.m()` -> `NEDevice.get().getToken("946be734f7a741f5b1f36970b3075c7f")`.

The next useful rooted-device step is to hook or log these Java calls:

```text
k21.b.F0()
s72.b.f288632a.m()
p62.e.requireKey(int, String, long)
com.aegis.sdk.AegisNative.onNetworkResponse(long, int, String)
```

This should reveal the exact public-key update plaintext before eapi wrapping
and the server response body after normal HTTP, without needing AB traffic for
regular user APIs.

## Current third-party implementation status

The known flow is complete enough to build functional third-party xeapi
requests, subject to normal account cookies and runtime anti-spam tokens.

1. Static initialization:
   - Decode or ship the corrected static AES key and sign key above.
   - Load `public_key` cache from
     `files/aegissdk/public_key`; it is Base64(JSON).
   - If no valid cache exists, call `/eapi/gorilla/anti/crawler/security/key/get`
     as a standard eapi request.

2. Public key refresh:
   - Native fields: `currentKeyVersion`, `timestamp`, 16-digit `nonce`,
     `requestType`, and
     `signature = base64(HMAC-SHA256(signKey, timestamp + nonce))`.
   - Java/network fields: `t1`, `t2`, `os`, `appVersion`, `deviceId`, `uid`,
     usually `e_r`, `header`, and anti-spam `checkToken`.
   - Submit with standard eapi `params=serialData(...)`.
   - Decrypt HTTP response with old eapi response decrypt, gzip if needed.
   - Verify response signature with the original request nonce.
   - Decrypt `data.encryptedData` using AES-256-ECB static key and store the
     Base64(JSON) public key cache.

3. xeapi request build:
   - Start from the original `/api/...` or `/eapi/...` URL and normalize
     `/eapi/ -> /api/`.
   - Build the plaintext JSON envelope as `n72.a.h()` does:
     `contentType` only for non-form bodies, `method` only when not POST,
     original encoded `queryString` when present, Base64 raw `body` when a
     request body exists, then append `e_r=true` into `queryString`.
   - Choose dynamic AES key:
     - if current session has both `x-encr-ssid` and `x-encr-sskey`, use the
       session key bytes as the dynamic key and put the session id in `R`.
     - otherwise generate a fresh 16-byte key and use empty session id.
   - `B = AES-ECB(dynamicKey, transform(AES-ECB(staticKey, plaintextJson)))`.
   - `S = X25519/AES-GCM(publicKey, base64(dynamicKey) + "|" + os + "|" + sk)`.
   - `R = AES-ECB(staticKey, version + "|" + sessionId)`.
   - Final form body is
     `B=<percent(base64(B))>&S=<percent(base64(S))>&R=<percent(base64(R))>`.
   - Send to final path `/xeapi/...` with `X-Client-Enc-State: ENCRYPTED` and
     normal app headers/cookies.

4. xeapi response:
   - The observed business response body for `/xeapi/song/enhance/location/info`
     is still the legacy eapi response wrapper, not an Aegis B/S/R response:
     `AES-128-ECB(e82ckenh8dichen8)`, then gzip if plaintext starts with gzip.
   - Response headers may carry `x-encr-ssid` and `x-encr-sskey`; when both are
     present, Java calls `AegisNative.setSession(id, key)` and later requests
     use that session key as the dynamic AES key.

Remaining important unknown:

- The exact encoding/length of `x-encr-sskey` is now observed. Shared captures
  include values such as `285e72792d1b18095684799c199c71a3` and
  `028defea4461d3e42070fafecd5cb9d1`: 32 hex-looking characters. Java passes
  the header string directly to native, and native uses its raw string bytes as
  the AES key. Therefore this is a 32-byte ASCII AES-256 key, not a hex-decoded
  16-byte AES-128 key.

Additional `/xeapi/nos/token/whalealloc` capture:

- Request `R=3LCoCTuHo/mDfZ1x3PtHsQ==` decrypts to:

```text
1000000000000|
```

  So this specific request did not use a prior session id while being built.
- Response headers include:

```text
x-encr-ssid: 6035f9409cb34196bfefd33afef8185c
x-encr-sskey: 285e72792d1b18095684799c199c71a3
```

  These are for subsequent requests.
- Response body again decrypts with the legacy eapi response wrapper. The
  plaintext is a normal NOS token allocation JSON with top-level `code=200` and
  data fields `bucket`, `docId`, `objectKey`, `outerUrl`, `resourceId`, and
  `token`. The token itself is a credential and should not be stored in notes.

Session continuity examples from captures:

- Previous response session pair:

```text
x-encr-ssid: 01c3a3532a884dd2a583228d6f335211
x-encr-sskey: 028defea4461d3e42070fafecd5cb9d1
```

- The later `/xeapi/song/enhance/location/info` request has `R` plaintext:

```text
1000000000000|01c3a3532a884dd2a583228d6f335211
```

  So that request was built while the above session id was active; its dynamic
  AES key should be the raw ASCII bytes of the matching `x-encr-sskey`.
- The `/xeapi/nos/token/whalealloc` request has `R` plaintext:

```text
1000000000000|
```

  So that request was built without an active session id, regardless of what
  session had been seen in another capture. It then received a new session pair
  in its own response headers.

The local helper `tools/xeapi_crypto.py` implements the above pieces. Run:

```text
venv/bin/python tools/xeapi_crypto.py --demo-capture
```

It validates the shared capture's `R` value and legacy response body:

```text
captured_R_plaintext 1000000000000|01c3a3532a884dd2a583228d6f335211
captured_response_plaintext {"code":200}
```

For runtime confirmation on device, use:

```text
frida -U -f com.netease.cloudmusic -l tools/frida/hook_aegis_runtime.js
```

This logs `AegisNative.initializeEngine`, plaintext passed to
`AegisNative.encrypt`, encrypted B/S/R output, `setSession`, key-refresh
callbacks, `k21.b.F0()`, and `s72.b.m()`.

</details>

<details>
<summary>工具代码，来自GPT5.5，使用python</summary>

```python
#!/usr/bin/env python3
"""Helpers for reproducing the confirmed parts of NCM Android xeapi/AegisSDK.

This is intentionally small and explicit so each piece can be compared with the
native routines noted in xeapi_notes.md.
"""

from __future__ import annotations

import argparse
import base64
import gzip
import hashlib
import hmac
import json
import os
import secrets
from dataclasses import dataclass
from typing import Optional
from urllib.parse import parse_qs, unquote, urlsplit

from cryptography.hazmat.primitives.asymmetric import x25519
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat


STATIC_KEY_B64 = "qx1aQw9rsEo/Aegd3XK9kW1c5ZEkisEocUgG1/j7G4Q="
STATIC_KEY = base64.b64decode(STATIC_KEY_B64)
SIGN_KEY = "mUHCwVNWJbunMqAHf5MImuirT6plvs6VSFW62MGHstFQxhBGdEoIhLItH3djc4+FB/OKty3+lL2rGeoFBpVe5g=="
EAPI_KEY = b"e82ckenh8dichen8"
EAPI_SEPARATOR = "-36cd479b6b5-"


def normalize_api_url_for_xeapi(url: str) -> str:
    return url.replace("/eapi/", "/api/", 1)


def final_xeapi_url(url: str) -> str:
    return normalize_api_url_for_xeapi(url).replace("/api/", "/xeapi/", 1)


def build_xeapi_plaintext_json(
    url: str,
    body: bytes | None = None,
    method: str = "POST",
    content_type: str | None = "application/x-www-form-urlencoded;charset=utf-8",
    e_r: bool = True,
) -> str:
    """Approximate the Java xeapi plaintext envelope built by n72.a.h().

    The returned compact JSON matches the field rules and insertion order seen
    in jadx. Exact string escaping is FastJSON-dependent, so captured plaintext
    remains the authority for byte-for-byte comparisons.
    """

    normalized_url = normalize_api_url_for_xeapi(url)
    fields: dict[str, str] = {}
    if content_type:
        media_type = content_type.split(";", 1)[0]
        if media_type.lower() != "application/x-www-form-urlencoded":
            fields["contentType"] = content_type
    if method.upper() != "POST":
        fields["method"] = method

    split_url = urlsplit(normalized_url)
    query_string = split_url.query
    if query_string:
        fields["queryString"] = query_string

    if body is not None:
        fields["body"] = base64.b64encode(body).decode()

    # n72.a.a(..., xeapi=true) appends e_r after body insertion. If an original
    # queryString exists, JSONObject keeps its original position while replacing
    # the value; otherwise queryString is inserted at the end.
    er_value = "true" if e_r else "false"
    if "queryString" in fields:
        fields["queryString"] = f"{fields['queryString']}&e_r={er_value}"
    else:
        fields["queryString"] = f"e_r={er_value}"
    return json.dumps(fields, separators=(",", ":"), ensure_ascii=False)


def pkcs7_pad(data: bytes, block_size: int = 16) -> bytes:
    pad_len = block_size - (len(data) % block_size)
    return data + bytes([pad_len]) * pad_len


def aes_ecb_encrypt(key: bytes, plaintext: bytes) -> bytes:
    encryptor = Cipher(algorithms.AES(key), modes.ECB()).encryptor()
    return encryptor.update(pkcs7_pad(plaintext)) + encryptor.finalize()


def pkcs7_unpad(data: bytes, block_size: int = 16) -> bytes:
    if not data or len(data) % block_size:
        raise ValueError("invalid PKCS#7 data length")
    pad_len = data[-1]
    if pad_len < 1 or pad_len > block_size:
        raise ValueError("invalid PKCS#7 padding length")
    if data[-pad_len:] != bytes([pad_len]) * pad_len:
        raise ValueError("invalid PKCS#7 padding bytes")
    return data[:-pad_len]


def aes_ecb_decrypt(key: bytes, ciphertext: bytes) -> bytes:
    decryptor = Cipher(algorithms.AES(key), modes.ECB()).decryptor()
    padded = decryptor.update(ciphertext) + decryptor.finalize()
    return pkcs7_unpad(padded)


def eapi_serial_data(api_path: str, body: str | dict, sort_keys: bool = False) -> str:
    """Standard NCM eapi params encryption.

    This mirrors NeteaseMusicUtils.serialdata(path, jsonString): MD5 signed
    payload, AES-128-ECB with key e82ckenh8dichen8, uppercase hex output.
    """

    text = (
        json.dumps(body, separators=(",", ":"), ensure_ascii=False, sort_keys=sort_keys)
        if isinstance(body, dict)
        else body
    )
    digest = hashlib.md5(f"nobody{api_path}use{text}md5forencrypt".encode()).hexdigest()
    plaintext = f"{api_path}{EAPI_SEPARATOR}{text}{EAPI_SEPARATOR}{digest}".encode()
    return aes_ecb_encrypt(EAPI_KEY, plaintext).hex().upper()


def eapi_deserial_data(hex_params: str) -> str:
    return aes_ecb_decrypt(EAPI_KEY, bytes.fromhex(hex_params)).decode()


def decrypt_legacy_eapi_response_body(ciphertext: bytes) -> bytes:
    """Decrypt old app response bodies used by eapi and observed xeapi replies.

    NeteaseMusicUtils.deserialdata() decrypts the response bytes with the
    standard eapi AES key. Captured xeapi business responses may still use this
    old response wrapper; if the decrypted plaintext is gzip, this helper also
    decompresses it.
    """

    plaintext = aes_ecb_decrypt(EAPI_KEY, ciphertext)
    if plaintext.startswith(b"\x1f\x8b"):
        return gzip.decompress(plaintext)
    return plaintext


def native_b64(data: bytes) -> bytes:
    return base64.b64encode(data)


def native_b64decode(text: str | bytes) -> bytes:
    return base64.b64decode(text)


def aegis_percent_encode(data: bytes | str) -> str:
    if isinstance(data, str):
        data = data.encode()
    keep = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
    out = []
    for b in data:
        if b in keep:
            out.append(chr(b))
        else:
            out.append(f"%{b:02X}")
    return "".join(out)


def aegis_mid_transform(ciphertext: bytes, r: Optional[bytes] = None) -> bytes:
    """Reproduce sub_12F4C4.

    Output is: 16 random bytes || rotated_base64(ciphertext XOR random bytes).
    The optional r argument is useful for deterministic tests.
    """

    if r is None:
        r = os.urandom(16)
    if len(r) != 16:
        raise ValueError("r must be exactly 16 bytes")
    xored = bytes(b ^ r[i & 0xF] for i, b in enumerate(ciphertext))
    encoded = native_b64(xored)
    if not encoded:
        return r
    rot = (r[0] & 0xF) % len(encoded)
    return r + encoded[rot:] + encoded[:rot]


def hkdf_aegis_x25519(shared_secret: bytes, ephemeral_public_key: bytes, length: int = 16) -> bytes:
    """HKDF-SHA256 as used by sub_12FBE0 in sub_12A968.

    Native call shape:
      PRK = HMAC-SHA256(zero32, shared_secret)
      OKM = HMAC-SHA256(PRK, T(prev) || ephemeral_public_key || counter)

    For the SDK's current length=16 only T(1) is needed.
    """

    if not shared_secret:
        shared_secret = b"\x00" * 32
    prk = hmac.new(b"\x00" * 32, shared_secret, hashlib.sha256).digest()
    out = b""
    prev = b""
    counter = 1
    while len(out) < length:
        prev = hmac.new(prk, prev + ephemeral_public_key + bytes([counter]), hashlib.sha256).digest()
        out += prev
        counter += 1
    return out[:length]


def update_public_key_signature(timestamp: int | str, nonce: str, sign_key: str = SIGN_KEY) -> str:
    """Signature check used for public-key update responses.

    Native computes Base64(HMAC-SHA256(signKey, str(responseTimestamp) + requestNonce)).
    """

    digest = hmac.new(sign_key.encode(), (str(timestamp) + nonce).encode(), hashlib.sha256).digest()
    return base64.b64encode(digest).decode()


def public_key_request_signature(timestamp: int | str, nonce: str, sign_key: str = SIGN_KEY) -> str:
    """Signature used in the native public-key update request payload.

    sub_12B8D4 computes Base64(HMAC-SHA256(signKey, str(timestamp) + nonce)).
    """

    digest = hmac.new(sign_key.encode(), (str(timestamp) + nonce).encode(), hashlib.sha256).digest()
    return base64.b64encode(digest).decode()


def generate_public_key_nonce() -> str:
    """Generate a native-shaped public-key update nonce.

    Native uses an MT19937 seeded from /dev/urandom and draws 16 values from
    uniform_int_distribution<int>(0, 9), then appends each digit.
    """

    return "".join(str(secrets.randbelow(10)) for _ in range(16))


def build_public_key_request_payload(
    current_key_version: str,
    timestamp: int | str,
    nonce: str,
    request_type: str = "active",
    sign_key: str = SIGN_KEY,
) -> str:
    """Build the compact JSON payload native sends to Java requireKey().

    Java then appends t1/t2/os/appVersion/deviceId/uid and submits the request.
    """

    if len(nonce) != 16 or not nonce.isdigit():
        raise ValueError("native public-key update nonce must be 16 decimal digits")

    payload = {
        "currentKeyVersion": current_key_version,
        "signature": public_key_request_signature(timestamp, nonce, sign_key),
        "timestamp": str(timestamp),
        "nonce": nonce,
        "requestType": request_type,
    }
    return json.dumps(payload, separators=(",", ":"), ensure_ascii=False)


def build_key_get_plaintext_params(
    current_key_version: str,
    timestamp: int | str,
    nonce: str,
    t1: str,
    t2: str,
    app_version: str,
    device_id: str,
    uid: int | str = 0,
    check_token: str | None = None,
    request_type: str = "active",
    sign_key: str = SIGN_KEY,
    include_eapi_fields: bool = True,
    sort_keys: bool = True,
) -> str:
    """Build the JSON plaintext Java passes to normal eapi serialdata().

    `t1`, `t2`, and `checkToken` are runtime SDK tokens:
      t1 = k21.b.F0() / WatchMan checkToken
      t2 = s72.b.m() / NEDevice fingerprint token
      checkToken = AntiSpamService.appendYdToken / X-antiCheatToken token

    This intentionally stops before NeteaseMusicUtils.serialdata(), which is
    the standard app eapi wrapper rather than an Aegis-specific primitive.
    """

    payload = json.loads(
        build_public_key_request_payload(
            current_key_version=current_key_version,
            timestamp=timestamp,
            nonce=nonce,
            request_type=request_type,
            sign_key=sign_key,
        )
    )
    payload.update(
        {
            "t1": t1,
            "t2": t2,
            "os": "android",
            "appVersion": app_version,
            "deviceId": device_id,
            "uid": str(uid),
        }
    )
    if check_token is not None:
        payload["checkToken"] = check_token
    if include_eapi_fields:
        payload["e_r"] = True
        payload["header"] = "{}"
    return json.dumps(payload, separators=(",", ":"), ensure_ascii=False, sort_keys=sort_keys)


def decrypt_public_key_response_inner(encrypted_data_b64: str, static_key: bytes = STATIC_KEY) -> bytes:
    """Decrypt data.encryptedData from the public-key update response."""

    return aes_ecb_decrypt(static_key, native_b64decode(encrypted_data_b64))


@dataclass(frozen=True)
class SEnvelope:
    raw: bytes
    ephemeral_public_key: bytes
    iv: bytes
    ciphertext: bytes
    tag: bytes


def encrypt_s(
    dynamic_key: bytes,
    os_name: str,
    sk: str,
    peer_public_key_b64: str,
    ephemeral_private_key: Optional[x25519.X25519PrivateKey] = None,
    iv: Optional[bytes] = None,
) -> SEnvelope:
    """Reproduce the S component before URL encoding.

    Native plaintext is base64(dynamic_key) + "|" + os + "|" + sk.
    """

    peer_public_key = x25519.X25519PublicKey.from_public_bytes(native_b64decode(peer_public_key_b64))
    private_key = ephemeral_private_key or x25519.X25519PrivateKey.generate()
    ephemeral_public_key = private_key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
    shared_secret = private_key.exchange(peer_public_key)
    aes_key = hkdf_aegis_x25519(shared_secret, ephemeral_public_key, 16)
    nonce = iv or os.urandom(12)
    if len(nonce) != 12:
        raise ValueError("iv must be exactly 12 bytes")
    plaintext = native_b64(dynamic_key) + b"|" + os_name.encode() + b"|" + sk.encode()
    encrypted = AESGCM(aes_key).encrypt(nonce, plaintext, None)
    ciphertext, tag = encrypted[:-16], encrypted[-16:]
    raw = ephemeral_public_key + nonce + ciphertext + tag
    return SEnvelope(raw=raw, ephemeral_public_key=ephemeral_public_key, iv=nonce, ciphertext=ciphertext, tag=tag)


def encrypt_b(plaintext_json: bytes, dynamic_key: bytes, r: Optional[bytes] = None) -> bytes:
    first = aes_ecb_encrypt(STATIC_KEY, plaintext_json)
    transformed = aegis_mid_transform(first, r)
    return aes_ecb_encrypt(dynamic_key, transformed)


def encrypt_r(version: str, session_id: str = "") -> bytes:
    """Reproduce R before Base64/URL encoding.

    Native sub_118BF0 builds:
      publicKeyVersion + "|" + currentSessionId
    then encrypts it with the static AES-256-ECB cipher.
    """

    return aes_ecb_encrypt(STATIC_KEY, f"{version}|{session_id}".encode())


def final_param_value(raw: bytes) -> str:
    """Final SDK output encodes raw component bytes as Base64, then percent-encodes."""

    return aegis_percent_encode(native_b64(raw))


def format_xeapi_body(b_raw: bytes, s_raw: bytes, r_raw: bytes) -> str:
    return f"B={final_param_value(b_raw)}&S={final_param_value(s_raw)}&R={final_param_value(r_raw)}"


@dataclass(frozen=True)
class PublicKeyState:
    public_key: str
    version: str
    next_update_time: int | None = None
    sk: str = ""

    @classmethod
    def from_json_bytes(cls, data: bytes) -> "PublicKeyState":
        obj = json.loads(data.decode())
        return cls(
            public_key=obj["publicKey"],
            version=str(obj["version"]),
            next_update_time=obj.get("nextUpdateTime"),
            sk=obj.get("sk", ""),
        )

    @classmethod
    def from_cache_b64(cls, cache_b64: str) -> "PublicKeyState":
        return cls.from_json_bytes(native_b64decode(cache_b64))


@dataclass(frozen=True)
class XeapiRequestParts:
    body: str
    plaintext_json: str
    dynamic_key: bytes
    b_raw: bytes
    s_raw: bytes
    r_raw: bytes


def build_xeapi_request_body(
    api_url: str,
    public_key_state: PublicKeyState,
    body: bytes | None = None,
    method: str = "POST",
    content_type: str | None = "application/x-www-form-urlencoded;charset=utf-8",
    os_name: str = "android",
    session_id: str = "",
    session_key: bytes | str | None = None,
    dynamic_key: bytes | None = None,
    transform_random: bytes | None = None,
    ephemeral_private_key: Optional[x25519.X25519PrivateKey] = None,
    s_iv: bytes | None = None,
) -> XeapiRequestParts:
    """Build a complete B/S/R form body for an /xeapi/* request.

    If a server session is known, pass both session_id and session_key. Native
    uses the session key as the dynamic AES key while still including S. Without
    a session key this generates a fresh 16-byte dynamic key.
    """

    if session_key is not None:
        active_dynamic_key = normalize_session_key(session_key)
    else:
        active_dynamic_key = dynamic_key or os.urandom(16)
    if len(active_dynamic_key) not in (16, 24, 32):
        raise ValueError("dynamic/session key must be a valid AES key length")

    plaintext = build_xeapi_plaintext_json(
        api_url,
        body=body,
        method=method,
        content_type=content_type,
        e_r=True,
    )
    b_raw = encrypt_b(plaintext.encode(), active_dynamic_key, transform_random)
    s_raw = encrypt_s(
        active_dynamic_key,
        os_name,
        public_key_state.sk,
        public_key_state.public_key,
        ephemeral_private_key=ephemeral_private_key,
        iv=s_iv,
    ).raw
    r_raw = encrypt_r(public_key_state.version, session_id)
    return XeapiRequestParts(
        body=format_xeapi_body(b_raw, s_raw, r_raw),
        plaintext_json=plaintext,
        dynamic_key=active_dynamic_key,
        b_raw=b_raw,
        s_raw=s_raw,
        r_raw=r_raw,
    )


def parse_xeapi_form_body(body: str) -> dict[str, bytes]:
    """Parse a B/S/R form body into raw component bytes."""

    parsed = parse_qs(body, keep_blank_values=True, strict_parsing=False)
    out: dict[str, bytes] = {}
    for name in ("B", "S", "R"):
        if name not in parsed or not parsed[name]:
            raise ValueError(f"missing {name}")
        out[name] = native_b64decode(unquote(parsed[name][0]))
    return out


def decrypt_r(raw_r: bytes) -> str:
    return aes_ecb_decrypt(STATIC_KEY, raw_r).decode()


def normalize_session_key(session_key: bytes | str) -> bytes:
    """Return the exact bytes native will use for Aegis session AES.

    Response header x-encr-sskey is passed from Java to AegisNative.setSession()
    as a string. Native stores that string and uses its bytes as the AES key.
    A 32-character hex-looking header is therefore a 32-byte ASCII AES-256 key,
    not a hex-decoded 16-byte AES-128 key.
    """

    if isinstance(session_key, str):
        return session_key.encode()
    return session_key


def demo_capture() -> None:
    """Check the public xeapi capture values that do not contain secrets."""

    captured_r = "6uMm/2V2SqT96D2FtoKGgFHzKX+TP+dChrWGTsVtcjBpuNxqLTfwHTEO8RThwA7e"
    captured_response_hex = (
        "BCC6C3A838364F78C6613EF403862326D0CB333FB97328516FB0C72CD7DB1B8E"
        "6AA3B102FBE7296AB0DB9EA5C46AD12B"
    )
    state = PublicKeyState.from_cache_b64(
        "eyJwdWJsaWNLZXkiOiIzbTV3TjlvbTExcVJFU2pFVis1RW9GZjlxTEV5bE82Z3lUaE1ibDFYeEVrPSIsInZlcnNpb24iOiIxMDAwMDAwMDAwMDAwIiwibmV4dFVwZGF0ZVRpbWUiOjE4MDM4ODIyNjkwMDAsInNrIjoiOFBaZmJJRkExNzc5OTQ0NDYzOTcyIn0="
    )
    parts = build_xeapi_request_body(
        "/api/song/enhance/location/info",
        state,
        body=b"",
        dynamic_key=bytes.fromhex("00112233445566778899aabbccddeeff"),
        transform_random=bytes(range(16)),
        s_iv=bytes(range(12)),
    )

    print("public_key_state", state)
    print("captured_R_plaintext", decrypt_r(native_b64decode(captured_r)))
    print("sample_sskey_len", len(normalize_session_key("285e72792d1b18095684799c199c71a3")))
    print("captured_response_plaintext", decrypt_legacy_eapi_response_body(bytes.fromhex(captured_response_hex)).decode())
    print("sample_xeapi_plaintext_json", parts.plaintext_json)
    print("sample_xeapi_body_len", len(parts.body))


def demo() -> None:
    dynamic_key = bytes.fromhex("00112233445566778899aabbccddeeff")
    body = json.dumps({"method": "GET", "url": "/api/test"}, separators=(",", ":")).encode()
    r = bytes(range(16))
    b_part = encrypt_b(body, dynamic_key, r)
    r_part = encrypt_r("1", "")
    print("static_key_hex", STATIC_KEY.hex())
    print("B_raw_b64", base64.b64encode(b_part).decode())
    print("R_raw_b64", base64.b64encode(r_part).decode())
    print("B_final_value", final_param_value(b_part))
    print("R_final_value", final_param_value(r_part))


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--demo", action="store_true")
    parser.add_argument("--demo-capture", action="store_true")
    args = parser.parse_args()
    if args.demo:
        demo()
    elif args.demo_capture:
        demo_capture()
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
```

</details>


<details>
<summary>如何获取最开始的公钥</summary>

我直接用了这个项目的/api发请求，发的data如下，这应该是我实验出来的最少要传的值，传的更少（或者省略掉下面的空值）就会报400
我这的timestamp，signature，nonce都是我自己的，真实请求需要参考上面文档换掉，贴在这是为了展示正确的样式
appVersion我这里实验可以从9.0.0-9.9.0,超过这个范围会报500:@AntiCrawlerSecurityKeyController 获取密钥异常：未找到匹配的密钥
```json
{
  "uri": "/api/gorilla/anti/crawler/security/key/get",
  "data": {
    "appVersion": "9.1.15",
    "currentKeyVersion": "",
    "deviceId": "",
    "nonce": "4477405878624231",
    "os": "android",
    "requestType": "active",
    "signature": "d6ouZ8bOiQrsH6kfslwG9zhJMvF6sJ4DCOlsGUkk7fw=",
    "t1": "",
    "t2": "",
    "timestamp": "1779955010033",
    "uid": ""
  },
  "crypto": "eapi"
}
```

目前似乎publickey是个定值，但sk每次获取都不一样
这里举一例
```json
{"publicKey":"3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=","version":"1000000000000","nextUpdateTime":1803882269000,"sk":"GYcibJw61779976227511"}
```

</details>