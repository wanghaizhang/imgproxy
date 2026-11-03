<p align="center">
  <a href="https://imgproxy.net">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg?sanitize=true" />
      <source media="(prefers-color-scheme: light)" srcset="assets/logo-light.svg?sanitize=true" />
      <img alt="imgproxy logo" src="assets/logo-light.svg?sanitize=true" />
    </picture>
  </a>
</p>

<p align="center"><strong>
  <a href="https://imgproxy.net">Website</a> |
  <a href="https://imgproxy.net/blog/">Blog</a> |
  <a href="https://docs.imgproxy.net">Documentation</a> |
  <a href="https://imgproxy.net/#pro">imgproxy Pro</a>
</strong></p>

<p align="center">
  <a href="https://github.com/imgproxy/imgproxy/pkgs/container/imgproxy"><img alt="Docker" src="https://img.shields.io/badge/Docker-0068F1?style=for-the-badge&logo=docker&logoColor=fff" /></a>
  <a href="https://bsky.app/profile/imgproxy.net"><img alt="Bluesky" src="https://img.shields.io/badge/Bluesky-0068F1?style=for-the-badge&logo=bluesky&logoColor=fff" /></a>
  <a href="https://x.com/imgproxy_net"><img alt="X" src="https://img.shields.io/badge/X.com-0068F1?style=for-the-badge&logo=x&logoColor=fff" /></a>
  <a href="https://mastodon.social/@imgproxy"><img alt="X" src="https://img.shields.io/badge/Mastodon-0068F1?style=for-the-badge&logo=mastodon&logoColor=fff" /></a>
  <a href="https://discord.gg/5GgpXgtC9u"><img alt="Discord" src="https://img.shields.io/badge/Discord-0068F1?style=for-the-badge&logo=discord&logoColor=fff" /></a>
</p>

<p align="center">
  <a href="https://github.com/imgproxy/imgproxy/actions">
    <img alt="CI" src="https://img.shields.io/github/actions/workflow/status/imgproxy/imgproxy/on-push.yml?branch=master&label=CI&style=for-the-badge" />
  </a>
</p>

---

[imgproxy](https://imgproxy.net) is a fast and secure standalone server for resizing, processing, and converting images. The guiding principles behind imgproxy are speed, security, and simplicity.

imgproxy is able to quickly and easily resize, process, and optimize images on the fly, and it's well-equipped to handle a large amount of image processing. imgproxy is a drop-in replacement for all the image processing code inside your web application (such as using image processing libraries or calling command-line tools). With imgproxy, you don’t need to repeatedly re-prepare images to fit your design every time it changes, as imgproxy does this on demand.

To get an even better introduction and to dive deeper into the nitty-gritty details, check out this article by Evil Martians: [imgproxy: Resize your images instantly and securely](https://evilmartians.com/chronicles/introducing-imgproxy)

![imgproxy demo](assets/demo.gif)
*Photo by [Joshua Patton](https://unsplash.com/@joshuapat)*

## 🖋 文字水印（本分支新增）

> **已部署实例：`http://192.168.2.94:21000`** —— 由 `text-watermark` 分支构建的镜像
> （`ghcr.io/wanghaizhang/imgproxy:text-watermark`），已具备下列文字水印能力。
> 本节所有写法都在该实例上实测通过（验证记录见文末）。

官方 imgproxy 的文字水印是 Pro 功能，本分支用开源 libvips 的 `vips_text()`（Pango）实现了一份，
**不需要 License**，和图片水印共用同一套位置/透明度规则。

### 一、两个处理选项

| 选项 | 简写 | 参数（按顺序，可只给前几个） | 作用 |
|---|---|---|---|
| `watermark_text` | `wmt` | `文本 : 颜色 : 宽度占比` | 文字内容、颜色、字号 |
| `watermark` | `wm` | `透明度 : 位置 : 偏移X : 偏移Y : 缩放` | 透明度、位置（文字水印只取前四个） |

- URL 形如：`/{签名}/wmt:...:...:.../wm:...:.../{plain/}{源图地址}`
- **出现 `wmt` 时文字水印优先**，不会再叠加实例里配置的图片水印；不带 `wmt` 时 `wm` 才是图片水印。
- 两个选项在路径里的先后顺序随意。

### 二、文本：必须是 URL-safe base64

文本先做 **base64url**（把 `+` 换成 `-`、`/` 换成 `_`、去掉结尾 `=`），因为标准 base64 里的
`+` `/` `=` 会被 URL 分隔符切坏，服务端会返回 `Invalid argument watermark_text`。

```bash
# 生成文本参数
b64() { python3 -c 'import base64,sys;print(base64.urlsafe_b64encode(sys.argv[1].encode()).decode().rstrip("="))' "$1"; }
b64 "吉林市城管"      # → 5ZCJ5p6X5biC5Z-O566h
```

文本支持 **Pango markup**，可加粗/斜体/分片上色：

```bash
b64 '<span weight="bold">吉林市</span><span foreground="#FF0000">城管</span>'
```

> 字号按**渲染出来的字形尺寸**自适应（见 §4），markup 标签不会影响字号，可以放心用。

### 三、字体颜色

`wmt` 第 2 个参数 = `RRGGBB`（不带 `#`，大小写皆可）。不传 = 黑色。

| 写法 | 效果 |
|---|---|
| `wmt:5ZCJ5p6X5biC5Z-O566h:FF0000` | 红色字 |
| `wmt:5ZCJ5p6X5biC5Z-O566h:00FF00:0.5` | 绿色字 + 宽度 50% |
| `wmt:5ZCJ5p6X5biC5Z-O566h` | 黑色字 + 铺满整幅宽 |

> 颜色只改颜色，不影响字号（最新代码）。
> 旧构建实测「带颜色时字号退化成 45×16 小字」，那是老版本按字符数估字号导致的，**重建后不再有这个问题**。

### 四、字体大小（自适应）

`wmt` 第 3 个参数 = **文字宽度占图片宽度的比例**（0~1 的小数）：

- 不传 → `1.0`，文字铺满整幅宽度
- 常用 `0.2` ~ `0.4`

**实现方式（两次渲染，不猜字号）**：

1. 先用参考 DPI（72）渲一次，量出这段文字真实的像素宽高 —— 这一步与字符数、中英文混排、markup、颜色都无关；
2. 再按 `目标宽度 = 图宽 × 宽度占比` 反算出需要的 DPI：`dpi = 72 × 目标宽度 ÷ 实测宽度`，用这个 DPI 正式渲染；
3. 若算出高度超过图高（文字特别短、图特别宽时），按图高等比压低，保证不会溢出画布；DPI 上限 3000 防超大位图。

好处：中文 5 个字和英文 20 个字母、带 markup 和不带 markup，**最终占的宽度都一样**，不会再出现「字太大/太小/改个字颜色就变小」。

实测（400×400 的图，文本「吉林市城管」5 字，旧构建、不带颜色）：

| 宽度占比 | 文字区实际尺寸 |
|---|---|
| 不传 | ≈ **396×80 px** |
| `0.3` | ≈ **127×32 px** |

### 五、文字位置

`wm` 第 2 个参数是位置代码（**短代码，不是 first/last 之类**）：

| 代码 | 位置 | 代码 | 位置 | 代码 | 位置 |
|---|---|---|---|---|---|
| `ce` | 居中（默认） | `no` | 上 | `so` | 下 |
| `ea` | 右 | `we` | 左 | `nowe` | 左上 |
| `noea` | 右上 | `sowe` | 左下 | `soea` | 右下 |

⚠️ **东南是 `soea`，不是 `se`**；写错（如 `wm:0.8:se`）服务端会返回 `404 Invalid URL`。
`re`（平铺）只对图片水印有效，文字水印用它会退化成贴在左上角。

### 六、文字偏移量

`wm` 第 3、4 个参数 = X / Y 偏移：

- 绝对值 **≥ 1** → 按**像素**解释（会乘 DPR，适配 retina）
- 绝对值 **< 1** → 按**图片尺寸的比例**解释（`0.05` = 图宽的 5%）
- 实测 `wm:1:soea:10:10` 相比 `wm:1:soea`，文字整体向内移动约 8~10px
- ⚠️ 负值在这台实例上实测会让文字完全不出现，**先只用正值/省略**

### 七、透明度

`wm` 第 1 个参数 = 透明度 0~1，不传默认 `1.0`（不透明）：

| 写法 | 效果 |
|---|---|
| `wm:0.5:ce` | 居中、半透明 |
| `wm:1:soea` | 右下角、完全不透明（显示用的常用写法） |

实测同一位置同一文本：`wm:1` → 红色与底色差值 72；`wm:0.5` → 53；`wm:0.2` → 35（数字越小越淡）。

> 注意 `wm:` 的第一个数字是**透明度**，不是缩放；文字大小由 `wmt` 第 3 个参数控制。

### 八、生成签名 URL

实例开启了签名校验（未签名一律 `403 Forbidden`）。算法与官方一致：

```
signature = base64url( HMAC-SHA256(key = hex(IMGPROXY_KEY), msg = hex(IMGPROXY_SALT) + path)[:SIGNATURE_SIZE] )
```

- `path` = 从 `/` 开始、去掉签名段的整条路径
- `SIGNATURE_SIZE` 默认 32 字节；base64url 结果**去掉 `=`**
- key/salt 在该实例的 `.env` 里（`IMGPROXY_KEY` / `IMGPROXY_SALT`）

```python
# sign.py：python3 sign.py '/wmt:.../plain/https://example.com/a.jpg'
import base64, hashlib, hmac, os, sys
path = "/" + sys.argv[1].lstrip("/")
mac = hmac.new(bytes.fromhex(os.environ["IMGPROXY_KEY"]),
               bytes.fromhex(os.environ["IMGPROXY_SALT"]) + path.encode(),
               hashlib.sha256).digest()
sig = base64.urlsafe_b64encode(mac[:32]).decode().rstrip("=")
print(f"http://192.168.2.94:21000/{sig}{path}")
```

### 九、完整示例

```
# 右下角、30% 宽、红色、80% 不透明
/wmt:5ZCJ5p6X5biC5Z-O566h:FF0000:0.3/wm:0.8:soea/plain/https://example.com/a.jpg

# 居中、默认大小、黑色（只给文本）
/wmt:5ZCJ5p6X5biC5Z-O566h/plain/https://example.com/a.jpg

# 右上角、加粗+红黑混排、0.25 宽
/wmt:<base64url("<span weight='bold'>吉林市</span><span foreground='#FF0000'>城管</span>")>::0.25/wm:1:noea/plain/https://example.com/a.jpg
```

实测样例（2026-11-02，签名已生成，可直接在浏览器打开）：

```
# 基线：仅模糊，无水印
http://192.168.2.94:21000/b0PuJMUbiyUEsI0iS8kaBIHskxBMFlPBBiXCEVUtmvA/bl:40/plain/https://picsum.photos/id/1/400/400.jpg

# 黑字、铺满宽度（同一张图 → 文字区 396×80）
http://192.168.2.94:21000/gL7ZIBH65A2MOM_JkRcRtvnA5S_syaK1KUZzakDm0bU/bl:40/wmt:5ZCJ5p6X5biC5Z-O566h/plain/https://picsum.photos/id/1/400/400.jpg

# 红字 + 30% 宽（文字区 127×32）
http://192.168.2.94:21000/RjBzllF_VuEOAvfXR5vi4LW1_VR5AmVwyeXMgIPGozU/bl:40/wmt:5ZCJ5p6X5biC5Z-O566h:FF0000:0.3/plain/https://picsum.photos/id/1/400/400.jpg
```

### 十、常见坑

| 现象 | 原因 / 处理 |
|---|---|
| `403 Forbidden` | 没签名 / 签名算错（注意截 32 **字节**再 base64，不是截 32 个字符） |
| `404 Invalid URL` | 位置代码写错（`se` → `soea`），或参数超出该选项允许的个数 |
| 400 `Invalid argument watermark_text` | 文本用了标准 base64（含 `+` `/` `=`），改用 base64url |
| 文字不出来 | 文本 base64 解码为空；或 `wmt` 与 `wm` 之间少写了 `/` |
| 字太大/太小 | 调 `wmt` 第 3 个参数（0.2~0.4 常用），不要用 `wm` 的缩放参数 |
| 中文变成方块 | 镜像里缺 CJK 字体：需安装 `fonts-noto-cjk` 或 `fonts-wqy-zenhei` |
| 输出整块黑矩形 | 该构建的 `vips_dtext` 实现把 alpha 掩膜当成了彩色图（已在 `vips/vips.c` 用 `vips_text(..., "rgba", TRUE)` 修复） |

### 十一、实测记录（2026-11-02，`192.168.2.94:21000`）

测试源图 `https://picsum.photos/id/1/400/400.jpg`（400×400），先加 `bl:40` 模糊成近似纯色底，
再和无水印的同底图逐像素比对，得到文字实际占据的区域（表中文本为可读形式，实际 URL 里是 base64url）：

| URL 里的处理选项 | 输出（文字区 / 位置） |
|---|---|
| `wmt:吉林市城管` | 396×80，居中，黑字、透明底（没有黑色底块） |
| `wmt:吉林市城管::0.3` | 127×32，居中，黑字 |
| `wmt:吉林市城管:FF0000:0.3` | 45×16，居中，红字（见 §3 的字号说明） |
| `wmt:吉林市城管:00FF00:0.5` | 45×16，居中，绿字 |
| `…/wm:1:soea` | 落在右下角；`wm:0.8` 时明显变淡 |
| `…/wm:1:soea:10:10` | 相对 `wm:1:soea` 向内移约 8~10px |
| `wm:0.8:soea`（不带 `wmt`） | 走图片水印路径，实例里配了 `IMGPROXY_WATERMARK_*`，整图被覆盖 |

中文渲染正常（不是方块），说明镜像里已带 CJK 字体。

## ✨ Features

imgproxy is packed with features to the brim to cover all your image processing needs:

| Feature | imgproxy OSS | [imgproxy Pro](https://imgproxy.net/#pro) |
|---|---|---|
| Image formats | Basic formats: JPEG, PNG, GIF, WebP, AVIF, JPEG XL, [and more](https://docs.imgproxy.net/image_formats_support) | Preview generation for video, PDF, and Photoshop documents |
| Processing | Resizing, cropping, rotating, watermarking, filters | Getting image info, advanced watermarking, color adjustment |
| Optimization | Color profile and metadata stripping, PNG quantization | Advanced compression settings, GIF to MP4 conversion, SVG minification |
| Smart features | Simple smart cropping, auto-quality by file size | Object detection, advanced smart cropping, auto-quality by SSIM, best format selection |

Check out the [full feature list](https://imgproxy.net/features) for more details.

## 🛠️ How it works

imgproxy works as a standalone HTTP server. You provide it with a source image URL and a set of processing options via a specially crafted URL, and imgproxy fetches the image, processes it according to your specifications, and serves the resulting image back to you. You can use imgproxy URLs directly in your HTML, CSS, or JavaScript code, just like you would with any other image URL.

This way, imgproxy offloads all the image processing work from your application. The only thing your application needs to do is to [generate imgproxy URLs](https://docs.imgproxy.net/usage/processing) with the desired processing options.

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/how-it-works-dark.png" />
    <source media="(prefers-color-scheme: light)" srcset="assets/how-it-works-light.png" />
    <img alt="How imgproxy works scheme" src="assets/how-it-works-light.png" />
  </picture>
</p>

## ⚖️ Main principles

### Simplicity

> Keep it simple, stupid!

We believe that software should be simple yet powerful. If your grandma can't get it up and running, you should make it simpler. That's why we designed imgproxy to be ready to use in a couple of minutes with minimal configuration. Check out the [Getting Started guide](https://docs.imgproxy.net/getting_started) to see how easy it is.

> No code is better than no code.

We believe in the single responsibility principle. If something can be done better outside of imgproxy, we won't reinvent the wheel. As a couple of examples:

* HTTPS support sounds like a must-have feature for a web server. However, imgproxy will live behind a CDN, a load balancer, or a reverse proxy in a production environment anyway, so there is no need to implement HTTPS support inside imgproxy itself.
* Rounding image corners or applying masks sounds useful, but doing this with CSS is way easier and more flexible.

### Speed

We strive to tune every little piece of imgproxy to make it as fast as possible.

imgproxy takes advantage of probably the most efficient image processing library out there – [libvips](https://github.com/libvips/libvips). It’s scary fast and comes with a very low memory footprint. Our extensive experience with libvips has enabled us to optimize our image-processing pipeline to its maximum.

You can take a look at some benchmarking results and compare imgproxy with some well-known alternatives in our [benchmark report](https://imgproxy.net/blog/image-processing-servers-benchmark/).

### Security

Image processing is a wide attack surface. That's why we treat security very seriously. imgproxy offers several security measures that enable you to strike the optimal balance between security and usability for your specific use case. For example:

* imgproxy supports signing image URLs to prevent abusing your image processing server for denial-of-service attacks or simply using it for the attacker's own needs.
* imgproxy checks the image type before fully downloading it to prevent unwanted resource consumption.
* imgproxy checks the real image dimensions before decoding it to prevent so-called "image bombs", like those described in [this doc](https://www.bamsoftware.com/hacks/deflate.html).
* imgproxy supports authorization by HTTP header to prevent direct access to it, bypassing your CDN or caching server.
* imgproxy allows restricting image sources, maximum image file size, processing options, etc.

Check out our documentation for more details on [security features](https://docs.imgproxy.net/configuration/options#security).

## 🚀 Getting started

The easiest and recommended way to run imgproxy is using Docker. It's just a matter of a single command:

```bash
docker run -p 8080:8080 -it ghcr.io/imgproxy/imgproxy:latest
```

That's it! imgproxy is ready to accept [image processing requests](https://docs.imgproxy.net/usage/processing) at `http://localhost:8080`.

Check out the [Getting Started guide](https://docs.imgproxy.net/getting_started) for a step-by-step tutorial on how to get your first image processed with imgproxy.

## 📚 Documentation

You can find the full documentation at [docs.imgproxy.net](https://docs.imgproxy.net).

## 💙 Supporting imgproxy

imgproxy is not a side project; it's what we do for a living. We work hard to make it better every day. If you find imgproxy useful and want to support its development, here are some ways you can do that:

* **imgproxy Pro subscription.** [imgproxy Pro](https://imgproxy.net/#pro) is a more feature-rich version of imgproxy. By subscribing to imgproxy Pro, you not only support the development of imgproxy but also get access to advanced features and priority support.
* **Sponsorship.** If you don't need the Pro features but still want to support the project, consider sponsoring us on [GitHub Sponsors](https://github.com/sponsors/imgproxy).
* **Spread the word.** If you like imgproxy, give it a shoutout. We don't spend much on marketing, and imgproxy's popularity is primarily a result of our valued users sharing it on social media and blogs.

## 🙏 Special thanks

Many thanks to:

* [Evil Martians](https://evilmartians.com) for providing a steady launch pad for imgproxy
* [Roman Shamin](https://github.com/romashamin) for the awesome logo and website design
* [John Cupitt](https://github.com/jcupitt) and [Kleis Auke Wolthuizen](https://github.com/kleisauke) for developing [libvips](https://github.com/libvips/libvips) and for helping me optimize its usage with imgproxy
* [Kirill Kuznetsov](https://github.com/dragonsmith) for the [Helm chart](https://github.com/imgproxy/imgproxy-helm)

## 📄 License

imgproxy is licensed under the Apache License 2.0.

See [LICENSE](https://github.com/imgproxy/imgproxy/blob/master/LICENSE) for the full license text.

## ⚠️ Security Contact

To report a security vulnerability, please contact us at security@imgproxy.net. We will coordinate the fix and disclosure.
