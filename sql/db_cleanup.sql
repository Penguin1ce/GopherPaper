-- 数据库精简迁移脚本(fix/db 分支)
-- AutoMigrate 只建表不删表,已存在的旧表与数据需手动清理。
-- 建议在备份后按顺序执行;每步幂等,可重复跑。

-- ── 1. paper_flows 并入 paper_reports ──────────────────────────
-- 思路图改存为 report_type='flow' 的一行,JSON 落 content(longtext)。
-- 先迁数据,再删旧表。旧表不存在则前两句可跳过。
INSERT INTO paper_reports (paper_id, report_type, content, created_at, updated_at)
SELECT paper_id, 'flow', flow_json, created_at, updated_at
FROM paper_flows
ON DUPLICATE KEY UPDATE content = VALUES(content), updated_at = VALUES(updated_at);

DROP TABLE IF EXISTS paper_flows;

-- ── 2. 移除后台运维/运营相关表(功能已下线)──────────────────────
-- 保留 admins(后台登录)与 service_call_logs(调用日志)。
DROP TABLE IF EXISTS admin_task_activities;
DROP TABLE IF EXISTS admin_task_checklist_items;
DROP TABLE IF EXISTS admin_task_comments;
DROP TABLE IF EXISTS admin_tasks;
DROP TABLE IF EXISTS announcements;
DROP TABLE IF EXISTS feedbacks;
DROP TABLE IF EXISTS admin_audit_logs;
DROP TABLE IF EXISTS system_settings;
DROP TABLE IF EXISTS system_model_configs;

-- ── 3. 移除未接通写入口的标签死功能 ────────────────────────────
-- 无任何创建/打标签路径,两表恒空,后台标签管理已一并下线。
DROP TABLE IF EXISTS paper_tags;
DROP TABLE IF EXISTS tags;
