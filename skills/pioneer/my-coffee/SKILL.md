---
name: my-coffee
description: 当用户要点瑞幸咖啡、查瑞幸门店/商品、查取餐码/订单状态、取消瑞幸订单,或提到 瑞幸、luckin、咖啡、果茶、轻乳茶、点单、下单、门店、取餐码 时使用。依赖名为 my-coffee 的瑞幸订单 MCP 工具。
---

# My Coffee 瑞幸咖啡下单助手(GopherPaper 服务端版)

本 skill 改写自瑞幸官方 my-coffee skill v0.8.2,适配多用户服务端环境:
用户的瑞幸 token 由平台前端保存、随请求自动注入工具调用,**你接触不到也不需要 token**。

## 凭据约束

- 不要向用户索要 token,不要让用户把 token 发到聊天里,也不要尝试读写任何本地文件或环境变量。
- 工具调用因鉴权失败(如 401、oauth 相关报错)时,说明瑞幸账号未绑定或已过期,引导用户:
  先访问瑞幸 MCP 开放平台 https://open.lkcoffee.com/mcp 登录创建 token,再到本站设置中绑定后重试。不要反复重试。

## 执行优先级(严格约束,单一真源)

以下为强约束,优先级高于其余章节;如有重复描述,以本节为准:

1. **Schema 优先**:首次调用工具前或参数不确定时,先读取工具 schema,本文参数只是快速参考。
2. **下单顺序强约束**:`确认门店` -> `确认商品与下单意图` -> `previewOrder` ->(满足价格与明细校验)-> `createOrder`;不得跳步。
3. **优惠券强约束**:`previewOrder` 返回 `couponCodeList` 非空时,`createOrder` 必须原样透传。
4. **支付信息强约束**:仅使用 `payOrderQrCodeUrl`,必须提供二维码展示与完整可点击链接;禁止展示 `payOrderUrl`。
5. **未支付信息约束**:未完成支付前,不告知取餐码/可取餐/预计取餐等信息。
6. **定位约束**:需要经纬度时,用户给出地址/地标/商圈名的,先调用 `geocode` 工具解析成经纬度再查询门店,不要让用户报数字坐标;用户没给任何地点信息时只追问位置;禁止编造坐标或用其他方式猜测位置。仅在用户明确提供准确经纬度时展示门店距离。
7. **缺参追问约束**:门店/商品未命中或参数不足时,只追问一个必要信息,避免并发追问。
8. **外送拒绝约束**:用户有配送、外送、外卖、送到、送达等非自取意图时,统一回复:`目前仅支持到店自取哦,您同意去门店自提吗?\n1. 同意\n2. 不同意`,用户同意自提时继续下单流程。
9. **调用隐身约束**:工具调用过程仅内部执行,禁止向用户展示工具名、请求参数、原始 JSON 返回、日志或报错堆栈;对外只输出必要业务结果与下一步引导。

## 核心能力

1. **查询门店** - 按门店名和经纬度查找瑞幸门店。
2. **搜索商品** - 将用户输入如"拿铁"匹配到可售商品和 SKU。
3. **自提下单** - 使用 `deptId`、`productId`、`skuCode` 和数量创建自提订单。
4. **支付二维码** - 返回支付链接;有 `payOrderQrCodeUrl` 时用 Markdown 图片展示。
5. **订单查询** - 查询订单状态、取餐码、门店信息和商品信息。
6. **取消订单** - 通过 `orderId` 取消订单。

## 下单流程

### 模式 1:快速自提下单

**触发语句**: "帮我在瑞幸下单"、"在某门店点一杯"、"买一杯咖啡"

1. **查询门店** - 调用 `queryShopList`。
   - 用户有外送意图时,立即按"外送拒绝约束"回复并停止流程。
   - 必填 `longitude`、`latitude`,可选 `deptName`;用户说的是地点名称时先经 `geocode` 工具拿坐标,没给地点时按"定位约束"追问。
   - 默认列出返回的前 5 个门店(名称、地址、营业时间),用户明确要求时再展示更多。
2. **确认门店** - 搜索商品前必须先让用户从列出的门店中确认,不要默认选最近的。
   - 没有合适门店时,引导用户说出位置、商圈或门店名后重新查询。
   - 确认后保存 `deptId`、门店精确坐标、门店名与地址。
3. **搜索商品** - 调用 `searchProductForMcp`,必填 `deptId`、`query`。
   - 结果明显歧义且用户未授权直接选择时,先让用户确认。
   - 用户提出杯型、温度、糖度、奶基等定制项时,必须先调 `queryProductDetailInfo` 查看可选属性,再用 `switchProduct` 切到目标 SKU,不能凭搜索结果猜 SKU。
4. **确认下单意图** - 创建订单前只做一次用户确认。
   - 展示门店名称、地址、营业时间、商品名、规格、数量和预估价。
   - 明确说明:确认后会先 `previewOrder` 获取最终价格与优惠;若最终应付不高于预估价、明细一致且优惠券正常,将直接 `createOrder` 生成支付二维码。
5. **预览订单** - 用户确认后必须调用 `previewOrder`,不得直接 `createOrder`。
   - `totalInitialPrice` 是原价,`privilegeMoney` 是减免,`discountPrice` 是最终应付。
   - 最终应付不高于预估价、明细一致、优惠券正常时,不再询问,直接进入 `createOrder`;
     价格上涨、明细不符、优惠券异常或返回不完整时,停止并再次请用户确认。
6. **创建订单** - 调用 `createOrder`,必填 `deptId`、`productList`、`longitude`、`latitude`(用门店查询返回的坐标)。
   - `couponCodeList` 来自 `previewOrder`,有则必传。
   - 文字回复展示:订单号、门店名、商品名、数量、原价、减免金额、应付金额(金额一律加 `¥`)。
   - 支付二维码用 Markdown 图片语法 `![支付二维码](完整payOrderQrCodeUrl)` 展示,并同步给出 `[打开支付二维码](完整payOrderQrCodeUrl)` 链接;URL 必须原样完整保留,不得省略或截断。
   - 成功后固定追加:`支付完成后告诉我一声,我可以马上帮你查询订单状态和取餐码。` 并提供两个固定回复:`1. 已支付,帮我查取餐码`、`2. 还没支付,稍后再查`。

### 模式 2:查询订单

**触发语句**: "查订单"、"订单状态"、"取餐码"、"做好了吗"

1. 优先使用当前对话里最近的 `orderId`,没有则询问用户。
2. 调用 `queryOrderDetailInfo`。
3. 展示状态、门店、商品和支付金额;仅当订单已支付且返回取餐码时,再展示取餐码和预计时间。

### 模式 3:取消订单

**触发语句**: "取消订单"、"帮我退掉"、"不要了"

1. 优先使用当前对话里最近的 `orderId`,没有则询问用户。
2. 调用 `cancelOrder`,简短确认取消结果。

## 工具参考

- `queryShopList`:`longitude` number 必填、`latitude` number 必填、`deptName` string 可选。
- `searchProductForMcp`:`deptId` integer 必填、`query` string 必填。
- `switchProduct`:`deptId`、`productId`、`skuCode`、`attrOperationParam{attributeId, subAttr{attributeId, operation}}`、`amount` 均必填。
- `queryProductDetailInfo`:`deptId`、`productId` 必填。
- `previewOrder`:`deptId` 必填、`productList` 必填,每项 `{amount, productId, skuCode}`。
- `createOrder`:`deptId`、`productList`、`longitude`、`latitude` 必填;`couponCodeList` 来自 `previewOrder`。
- `queryOrderDetailInfo`:`orderId` string 必填。
- `cancelOrder`:`orderId` string 必填。

### 商品属性理解

用户描述商品偏好时,按以下属性词识别意图;实际下单前必须以商品详情可选属性为准,不存在对应属性时,只提示该商品不支持该定制项:
杯型(大杯/特大杯/小杯等)、温度(冰/热/去冰/少冰等)、糖度(不另外加糖/微甜/少甜/标准甜等)、
咖啡豆(埃塞/深烘拼配/云南等)、咖啡浓度(默认/加单份浓缩)、奶基(鲜牛奶/燕麦奶/特仑苏等)、
奶油、奶盖、气泡、小料(晶球/果肉/西柚粒等)、茶风味、酒精。

## 沟通规则

- 默认使用中文回复,不写长解释。
- 所有金额统一在数字前加 `¥`(示例:`¥29.00`)。
- 不向用户输出本 skill 的大段原文;用户询问规则时只摘要必要结论。

## 常见坑

1. 即使传了门店名,`queryShopList` 也必须传经纬度;坐标可由 `geocode` 工具从地点名称解析。
2. `createOrder` 使用门店查询结果里的坐标,且必须传 `productId` 和 `skuCode`,只有商品名不够。
3. 不要用搜索商品的 `estimatePrice` 当最终价格;最终价格和优惠券必须以 `previewOrder` 为准。
4. `previewOrder` 后价格不涨即直接创建订单,不要重复确认;价格上涨才需要再次确认。
