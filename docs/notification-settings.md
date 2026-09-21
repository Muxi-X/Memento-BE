# 互动通知与创作提醒：前端对接

创作提醒的产品时间为每天中午 **12:00**。后端保存账号级开关，前端根据开关注册或取消手机本地通知。本次不增加服务端定时任务或远程推送，也不生成站内创作提醒消息。

## 读取设置

`GET /v1/me/settings`

请求头：`Authorization: Bearer <access_token>`。

成功返回 `200`，响应没有额外的 `data` 包装：

```json
{
  "profile": {
    "nickname": "示例用户"
  },
  "notifications": {
    "reaction_enabled": true,
    "creation_reminder_enabled": true
  }
}
```

`profile` 沿用现有结构，也可能返回 `email`、`avatar_url`。两个通知字段始终返回布尔值，数据库默认值均为 `true`；读取已有用户时返回实际保存的值，不覆盖用户偏好。

| 字段 | 含义 |
| --- | --- |
| `reaction_enabled` | 是否为该用户收到的灵感/共鸣互动生成站内消息 |
| `creation_reminder_enabled` | 是否启用每天 12:00 的创作提醒，由前端安排本地通知 |

## 修改设置

`PATCH /v1/me/settings/notifications`

请求头：`Authorization: Bearer <access_token>`、`Content-Type: application/json`。可选传入 `X-Request-ID`、`Idempotency-Key`，重复提交相同设置不会反转开关。

只关闭创作提醒：

```json
{
  "creation_reminder_enabled": false
}
```

同时修改两个开关：

```json
{
  "reaction_enabled": false,
  "creation_reminder_enabled": true
}
```

- 至少传入其中一个字段，未传的字段保持原值；`false` 表示关闭，不能当作未传。
- 仅支持布尔值，不接受 `null`、字符串、数字、空对象或未知字段。
- 成功返回 `200` 和完整的设置对象，结构与 GET 相同。
- `401` 表示未登录或令牌无效；`404` 表示用户设置不存在；`400` 表示请求不符合约定。错误响应沿用现有统一格式。
- 只修改当前登录用户的设置，不接收 `user_id`。
- 旧客户端继续只发送 `reaction_enabled` 即可，不会改动创作提醒。

## 前端配合事项

1. 登录、切换账号或 App 回到前台时读取设置，将返回的开关与本机通知安排同步。
2. 开启创作提醒时检查系统通知权限，安排每日 12:00 的本地通知并保存其标识，避免重复注册。
3. 关闭提醒或退出账号时取消对应的本地通知；修改失败时按服务器实际状态恢复界面并重新同步。
4. 数据库开关为 `true` 不代表手机已授权通知。系统权限不足或调度失败时，界面应说明本机提醒未生效。
5. 本接口不保存提醒时间、设备时区或系统通知权限。前端需与产品统一“手机当地 12:00”或“北京时间 12:00”的口径，避免混用；后端现有业务日期采用 `Asia/Shanghai`，不等同于本地通知已绑定该时区。
6. 账号级设置通过接口同步；其他设备已安排的本地通知，要在各自同步后更新。本地提醒不会加入 `/v1/me/notifications`，也不会增加 `unread_notification_count`。

## 部署与验证

`user_settings.creation_reminder_enabled` 已由 `000001_core.up.sql` 定义，本次无需新增数据库迁移。OpenAPI 与 sqlc 生成代码已同步更新。

后端测试覆盖默认值、两个开关独立开关及同时修改、显式 `false`、持久化、并发更新不覆盖、用户隔离、鉴权和非法请求。手机通知的权限、12:00 调度、点击跳转以及账号切换行为需由前端在真机验收。
