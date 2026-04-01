use std::{
  collections::HashMap,
  fs,
  path::{Path, PathBuf},
};

use rusqlite::{params, Connection};
use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Manager};

const DB_FILE_NAME: &str = "storage_nodes_mock.sqlite3";

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StorageNodesDashboard {
  nodes: Vec<StorageNodeRecord>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StorageNodeRecord {
  id: String,
  name: String,
  node_type: String,
  address: String,
  mount_mode: String,
  enabled: bool,
  scan_status: String,
  scan_tone: String,
  last_scan_at: String,
  heartbeat_policy: String,
  next_heartbeat_at: String,
  last_heartbeat_result: String,
  heartbeat_failures: i64,
  capacity_summary: String,
  free_space_summary: String,
  capacity_percent: i64,
  library_bindings: Vec<String>,
  badges: Vec<String>,
  risk_tags: Vec<String>,
  auth_status: String,
  auth_tone: String,
  notes: String,
  detail: StorageNodeDetail,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "camelCase")]
enum StorageNodeDetail {
  Local { root_path: String },
  Nas {
    protocol: String,
    host: String,
    share_name: String,
    username: String,
    password_hint: String,
  },
  Cloud {
    vendor: String,
    account_alias: String,
    mount_directory: String,
    access_method: String,
    qr_channel: Option<String>,
    token_status: String,
  },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StorageNodeDraft {
  id: Option<String>,
  name: String,
  node_type: String,
  notes: String,
  mount_mode: String,
  heartbeat_policy: String,
  detail: StorageDraftDetail,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "camelCase")]
enum StorageDraftDetail {
  Local { root_path: String },
  Nas {
    protocol: String,
    host: String,
    share_name: String,
    username: String,
    password: String,
  },
  Cloud {
    vendor: String,
    account_alias: String,
    mount_directory: String,
    access_method: String,
    qr_channel: String,
    token: String,
  },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StorageCredentialDraft {
  id: String,
  node_name: String,
  auth_mode: String,
  username: String,
  password: String,
  token: String,
  qr_channel: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StorageScanHistoryItem {
  id: String,
  started_at: String,
  finished_at: String,
  status: String,
  summary: String,
  trigger: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ScanHistoryResponse {
  node_id: String,
  items: Vec<StorageScanHistoryItem>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MessageResponse {
  message: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ConnectionCheck {
  label: String,
  status: String,
  detail: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StorageConnectionTestResult {
  node_id: String,
  node_name: String,
  overall_tone: String,
  summary: String,
  checks: Vec<ConnectionCheck>,
  suggestion: String,
  tested_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ConnectionTestResponse {
  message: String,
  results: Vec<StorageConnectionTestResult>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct IdsPayload {
  ids: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct UpdateHeartbeatPayload {
  ids: Vec<String>,
  heartbeat_policy: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct UpdateEnabledPayload {
  ids: Vec<String>,
  enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct IdPayload {
  id: String,
}

#[tauri::command]
fn storage_nodes_load_dashboard(app: AppHandle) -> Result<StorageNodesDashboard, String> {
  let conn = open_database(&app)?;
  Ok(StorageNodesDashboard { nodes: load_nodes(&conn)? })
}

#[tauri::command]
fn storage_nodes_save_node(app: AppHandle, draft: StorageNodeDraft) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let record = draft_to_record(draft);
  upsert_node(&conn, &record)?;
  Ok(MessageResponse { message: "存储节点已保存".into() })
}

#[tauri::command]
fn storage_nodes_run_scan(app: AppHandle, payload: IdsPayload) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut nodes = load_nodes(&conn)?;

  for node in nodes.iter_mut().filter(|node| payload.ids.contains(&node.id)) {
    node.scan_status = "扫描中".into();
    node.scan_tone = "warning".into();
    node.last_scan_at = "正在执行".into();
    upsert_node(&conn, node)?;
    prepend_history(
      &conn,
      &node.id,
      StorageScanHistoryItem {
        id: format!("history-{}", &node.id),
        started_at: "刚刚".into(),
        finished_at: "进行中".into(),
        status: "进行中".into(),
        summary: "扫描任务已创建，可继续浏览其他节点。".into(),
        trigger: if payload.ids.len() > 1 { "批量扫描".into() } else { "手动扫描".into() },
      },
    )?;
  }

  Ok(MessageResponse {
    message: format!("已为 {} 个节点创建扫描任务", payload.ids.len()),
  })
}

#[tauri::command]
fn storage_nodes_run_connection_test(
  app: AppHandle,
  payload: IdsPayload,
) -> Result<ConnectionTestResponse, String> {
  let conn = open_database(&app)?;
  let nodes = load_nodes(&conn)?;
  let results = build_connection_results(&nodes, &payload.ids);

  Ok(ConnectionTestResponse {
    message: if payload.ids.len() > 1 {
      format!("已完成 {} 个节点的连接测试", payload.ids.len())
    } else {
      "连接测试已完成".into()
    },
    results,
  })
}

#[tauri::command]
fn storage_nodes_update_heartbeat(
  app: AppHandle,
  payload: UpdateHeartbeatPayload,
) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut nodes = load_nodes(&conn)?;

  for node in nodes.iter_mut().filter(|node| payload.ids.contains(&node.id)) {
    node.heartbeat_policy = payload.heartbeat_policy.clone();
    node.next_heartbeat_at = next_heartbeat_text(&payload.heartbeat_policy).into();
    if payload.heartbeat_policy == "从不" {
      node.last_heartbeat_result = "无需心跳".into();
    }
    upsert_node(&conn, node)?;
  }

  Ok(MessageResponse {
    message: if payload.ids.len() > 1 {
      format!("已更新 {} 个节点的心跳策略", payload.ids.len())
    } else {
      "心跳策略已更新".into()
    },
  })
}

#[tauri::command]
fn storage_nodes_save_credentials(
  app: AppHandle,
  draft: StorageCredentialDraft,
) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut nodes = load_nodes(&conn)?;

  if let Some(node) = nodes.iter_mut().find(|node| node.id == draft.id) {
    node.auth_status = "鉴权正常".into();
    node.auth_tone = "success".into();
    node.risk_tags.retain(|tag| tag != "鉴权异常");

    match &mut node.detail {
      StorageNodeDetail::Nas { username, password_hint, .. } => {
        *username = draft.username;
        *password_hint = if draft.password.is_empty() { "未更新" } else { "刚刚更新" }.into();
      }
      StorageNodeDetail::Cloud { token_status, access_method, qr_channel, .. } => {
        *access_method = draft.auth_mode.clone();
        *qr_channel = if draft.auth_mode == "扫码登录获取 Token" {
          Some(draft.qr_channel.clone())
        } else {
          None
        };
        *token_status = if draft.token.is_empty() { "未配置" } else { "已更新" }.into();
      }
      StorageNodeDetail::Local { .. } => {}
    }

    upsert_node(&conn, node)?;
  }

  Ok(MessageResponse { message: "鉴权信息已保存".into() })
}

#[tauri::command]
fn storage_nodes_update_enabled(
  app: AppHandle,
  payload: UpdateEnabledPayload,
) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut nodes = load_nodes(&conn)?;

  for node in nodes.iter_mut().filter(|node| payload.ids.contains(&node.id)) {
    node.enabled = payload.enabled;
    upsert_node(&conn, node)?;
  }

  Ok(MessageResponse {
    message: if payload.enabled { "已启用所选节点".into() } else { "已停用所选节点".into() },
  })
}

#[tauri::command]
fn storage_nodes_delete_node(app: AppHandle, payload: IdPayload) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  conn
    .execute("DELETE FROM storage_nodes WHERE id = ?1", params![payload.id])
    .map_err(|error| error.to_string())?;
  conn
    .execute("DELETE FROM storage_scan_history WHERE node_id = ?1", params![payload.id])
    .map_err(|error| error.to_string())?;

  Ok(MessageResponse { message: "节点已删除".into() })
}

#[tauri::command]
fn storage_nodes_load_scan_history(
  app: AppHandle,
  payload: IdPayload,
) -> Result<ScanHistoryResponse, String> {
  let conn = open_database(&app)?;
  Ok(ScanHistoryResponse {
    node_id: payload.id.clone(),
    items: load_histories(&conn, &payload.id)?,
  })
}

fn open_database(app: &AppHandle) -> Result<Connection, String> {
  let path = db_path(app)?;
  if let Some(parent) = path.parent() {
    fs::create_dir_all(parent).map_err(|error| error.to_string())?;
  }

  let conn = Connection::open(path).map_err(|error| error.to_string())?;
  init_schema(&conn)?;
  seed_if_needed(&conn)?;
  ensure_database_compatible(&conn)?;
  Ok(conn)
}

fn db_path(app: &AppHandle) -> Result<PathBuf, String> {
  let base = app.path().app_data_dir().map_err(|error| error.to_string())?;
  Ok(base.join(Path::new(DB_FILE_NAME)))
}

fn init_schema(conn: &Connection) -> Result<(), String> {
  conn
    .execute_batch(
      "CREATE TABLE IF NOT EXISTS storage_nodes (id TEXT PRIMARY KEY, payload TEXT NOT NULL);
       CREATE TABLE IF NOT EXISTS storage_scan_history (
         id TEXT PRIMARY KEY,
         node_id TEXT NOT NULL,
         payload TEXT NOT NULL
       );",
    )
    .map_err(|error| error.to_string())
}

fn seed_if_needed(conn: &Connection) -> Result<(), String> {
  let count: i64 = conn
    .query_row("SELECT COUNT(*) FROM storage_nodes", [], |row| row.get(0))
    .map_err(|error| error.to_string())?;

  if count > 0 {
    return Ok(());
  }

  for node in seed_nodes() {
    upsert_node(conn, &node)?;
  }

  for (node_id, items) in seed_histories() {
    for item in items {
      insert_history(conn, &node_id, &item)?;
    }
  }

  Ok(())
}

fn ensure_database_compatible(conn: &Connection) -> Result<(), String> {
  match load_nodes(conn) {
    Ok(nodes) if !nodes.is_empty() => Ok(()),
    _ => {
      conn
        .execute("DELETE FROM storage_nodes", [])
        .map_err(|error| error.to_string())?;
      conn
        .execute("DELETE FROM storage_scan_history", [])
        .map_err(|error| error.to_string())?;

      for node in seed_nodes() {
        upsert_node(conn, &node)?;
      }

      for (node_id, items) in seed_histories() {
        for item in items {
          insert_history(conn, &node_id, &item)?;
        }
      }

      Ok(())
    }
  }
}

fn load_nodes(conn: &Connection) -> Result<Vec<StorageNodeRecord>, String> {
  let mut stmt = conn
    .prepare("SELECT payload FROM storage_nodes ORDER BY rowid ASC")
    .map_err(|error| error.to_string())?;
  let rows = stmt
    .query_map([], |row| row.get::<_, String>(0))
    .map_err(|error| error.to_string())?;

  rows
    .into_iter()
    .map(|row| {
      let payload = row.map_err(|error| error.to_string())?;
      serde_json::from_str::<StorageNodeRecord>(&payload).map_err(|error| error.to_string())
    })
    .collect()
}

fn upsert_node(conn: &Connection, node: &StorageNodeRecord) -> Result<(), String> {
  let payload = serde_json::to_string(node).map_err(|error| error.to_string())?;
  conn
    .execute(
      "INSERT INTO storage_nodes (id, payload) VALUES (?1, ?2)
       ON CONFLICT(id) DO UPDATE SET payload = excluded.payload",
      params![node.id, payload],
    )
    .map_err(|error| error.to_string())?;
  Ok(())
}

fn load_histories(conn: &Connection, node_id: &str) -> Result<Vec<StorageScanHistoryItem>, String> {
  let mut stmt = conn
    .prepare("SELECT payload FROM storage_scan_history WHERE node_id = ?1 ORDER BY rowid DESC")
    .map_err(|error| error.to_string())?;
  let rows = stmt
    .query_map(params![node_id], |row| row.get::<_, String>(0))
    .map_err(|error| error.to_string())?;

  rows
    .into_iter()
    .map(|row| {
      let payload = row.map_err(|error| error.to_string())?;
      serde_json::from_str::<StorageScanHistoryItem>(&payload).map_err(|error| error.to_string())
    })
    .collect()
}

fn insert_history(conn: &Connection, node_id: &str, item: &StorageScanHistoryItem) -> Result<(), String> {
  let payload = serde_json::to_string(item).map_err(|error| error.to_string())?;
  conn
    .execute(
      "INSERT OR REPLACE INTO storage_scan_history (id, node_id, payload) VALUES (?1, ?2, ?3)",
      params![item.id, node_id, payload],
    )
    .map_err(|error| error.to_string())?;
  Ok(())
}

fn prepend_history(conn: &Connection, node_id: &str, item: StorageScanHistoryItem) -> Result<(), String> {
  insert_history(conn, node_id, &item)
}

fn build_connection_results(nodes: &[StorageNodeRecord], ids: &[String]) -> Vec<StorageConnectionTestResult> {
  ids
    .iter()
    .filter_map(|id| nodes.iter().find(|node| &node.id == id))
    .map(|node| StorageConnectionTestResult {
      node_id: node.id.clone(),
      node_name: node.name.clone(),
      overall_tone: if node.auth_tone == "critical" {
        "critical".into()
      } else if node.auth_tone == "warning" || !node.risk_tags.is_empty() {
        "warning".into()
      } else {
        "success".into()
      },
      summary: if node.auth_tone == "warning" || !node.risk_tags.is_empty() {
        "当前连接可达，但仍建议先处理风险提示。".into()
      } else {
        "节点可达且当前配置可继续使用。".into()
      },
      checks: vec![
        ConnectionCheck { label: "可达性".into(), status: "success".into(), detail: format!("{} 可达。", node.address) },
        ConnectionCheck { label: "鉴权状态".into(), status: node.auth_tone.clone(), detail: node.auth_status.clone() },
        ConnectionCheck { label: "读权限".into(), status: "success".into(), detail: "可读取目标目录。".into() },
        ConnectionCheck {
          label: "写权限".into(),
          status: if node.mount_mode == "只读" { "warning".into() } else { "success".into() },
          detail: if node.mount_mode == "只读" { "当前为只读挂载。".into() } else { "可写入目标目录。".into() },
        },
        ConnectionCheck { label: "目标目录可访问".into(), status: "success".into(), detail: "目录检查通过。".into() },
      ],
      suggestion: if node.auth_tone == "warning" || node.auth_tone == "critical" { "重新鉴权".into() } else { "可立即执行扫描".into() },
      tested_at: "刚刚".into(),
    })
    .collect()
}

fn draft_to_record(draft: StorageNodeDraft) -> StorageNodeRecord {
  let id = draft.id.unwrap_or_else(|| format!("node-{}", draft.name.replace(' ', "-")));
  let mount_mode = draft.mount_mode.clone();
  let heartbeat_policy = draft.heartbeat_policy.clone();

  match draft.detail {
    StorageDraftDetail::Local { root_path } => StorageNodeRecord {
      id,
      name: draft.name,
      node_type: draft.node_type,
      address: root_path.clone(),
      mount_mode: mount_mode.clone(),
      enabled: true,
      scan_status: "未扫描".into(),
      scan_tone: "info".into(),
      last_scan_at: "未扫描".into(),
      heartbeat_policy: heartbeat_policy.clone(),
      next_heartbeat_at: next_heartbeat_text(&heartbeat_policy).into(),
      last_heartbeat_result: if heartbeat_policy == "从不" { "无需心跳".into() } else { "尚未执行".into() },
      heartbeat_failures: 0,
      capacity_summary: "待首次检测".into(),
      free_space_summary: "待首次检测".into(),
      capacity_percent: 0,
      library_bindings: Vec::new(),
      badges: vec![mount_mode],
      risk_tags: Vec::new(),
      auth_status: "无需鉴权".into(),
      auth_tone: "info".into(),
      notes: draft.notes,
      detail: StorageNodeDetail::Local { root_path },
    },
    StorageDraftDetail::Nas { protocol, host, share_name, username, password } => StorageNodeRecord {
      id,
      name: draft.name,
      node_type: draft.node_type,
      address: format!("\\\\{}\\{}", host, share_name),
      mount_mode: mount_mode.clone(),
      enabled: true,
      scan_status: "未扫描".into(),
      scan_tone: "info".into(),
      last_scan_at: "未扫描".into(),
      heartbeat_policy: heartbeat_policy.clone(),
      next_heartbeat_at: next_heartbeat_text(&heartbeat_policy).into(),
      last_heartbeat_result: "尚未执行".into(),
      heartbeat_failures: 0,
      capacity_summary: "待首次检测".into(),
      free_space_summary: "待首次检测".into(),
      capacity_percent: 0,
      library_bindings: Vec::new(),
      badges: vec![mount_mode, "SMB".into()],
      risk_tags: Vec::new(),
      auth_status: if username.is_empty() { "待鉴权".into() } else { "待连接测试".into() },
      auth_tone: if username.is_empty() { "warning".into() } else { "info".into() },
      notes: draft.notes,
      detail: StorageNodeDetail::Nas {
        protocol,
        host,
        share_name,
        username,
        password_hint: if password.is_empty() { "未保存".into() } else { "刚刚更新".into() },
      },
    },
    StorageDraftDetail::Cloud { vendor, account_alias, mount_directory, access_method, qr_channel, token } => StorageNodeRecord {
      id,
      name: draft.name,
      node_type: draft.node_type,
      address: mount_directory.clone(),
      mount_mode: mount_mode.clone(),
      enabled: true,
      scan_status: "未扫描".into(),
      scan_tone: "info".into(),
      last_scan_at: "未扫描".into(),
      heartbeat_policy: heartbeat_policy.clone(),
      next_heartbeat_at: next_heartbeat_text(&heartbeat_policy).into(),
      last_heartbeat_result: "尚未执行".into(),
      heartbeat_failures: 0,
      capacity_summary: "远端容量待检测".into(),
      free_space_summary: "待首次检测".into(),
      capacity_percent: 0,
      library_bindings: Vec::new(),
      badges: vec!["115".into(), mount_mode],
      risk_tags: if token.is_empty() { vec!["鉴权异常".into()] } else { Vec::new() },
      auth_status: if access_method == "扫码登录获取 Token" {
        if token.is_empty() {
          format!("待完成{}扫码登录", qr_channel)
        } else {
          format!("已通过{}获取 Token", qr_channel)
        }
      } else if token.is_empty() {
        "Token 缺失".into()
      } else {
        "待连接测试".into()
      },
      auth_tone: if token.is_empty() { "warning".into() } else { "info".into() },
      notes: draft.notes,
      detail: StorageNodeDetail::Cloud {
        vendor,
        account_alias,
        mount_directory,
        access_method,
        qr_channel: Some(qr_channel),
        token_status: if token.is_empty() { "未配置".into() } else { "已配置".into() },
      },
    },
  }
}

fn next_heartbeat_text(policy: &str) -> &'static str {
  match policy {
    "每小时" => "1 小时后",
    "每日（深夜）" => "今晚 02:00",
    "每周（深夜）" => "周六 02:00",
    _ => "—",
  }
}

fn seed_nodes() -> Vec<StorageNodeRecord> {
  vec![
    StorageNodeRecord {
      id: "node-local-main".into(),
      name: "本地 NVMe 主盘".into(),
      node_type: "本机磁盘".into(),
      address: "D:\\Mare\\Assets".into(),
      mount_mode: "可写".into(),
      enabled: true,
      scan_status: "最近扫描成功".into(),
      scan_tone: "success".into(),
      last_scan_at: "今天 09:12".into(),
      heartbeat_policy: "从不".into(),
      next_heartbeat_at: "—".into(),
      last_heartbeat_result: "无需心跳".into(),
      heartbeat_failures: 0,
      capacity_summary: "已用 64% · 3.4 TB 可用".into(),
      free_space_summary: "3.4 TB 可用".into(),
      capacity_percent: 64,
      library_bindings: vec!["商业摄影资产库".into(), "视频工作流资产库".into()],
      badges: vec!["可写".into(), "已绑定 2 个资产库".into()],
      risk_tags: vec![],
      auth_status: "无需鉴权".into(),
      auth_tone: "info".into(),
      notes: "本地生产主盘".into(),
      detail: StorageNodeDetail::Local { root_path: "D:\\Mare\\Assets".into() },
    },
    StorageNodeRecord {
      id: "node-removable".into(),
      name: "现场移动硬盘 T7".into(),
      node_type: "本机磁盘".into(),
      address: "E:\\shooting-card".into(),
      mount_mode: "只读".into(),
      enabled: true,
      scan_status: "未扫描".into(),
      scan_tone: "info".into(),
      last_scan_at: "未扫描".into(),
      heartbeat_policy: "从不".into(),
      next_heartbeat_at: "—".into(),
      last_heartbeat_result: "无需心跳".into(),
      heartbeat_failures: 0,
      capacity_summary: "已用 82% · 1.2 TB 可用".into(),
      free_space_summary: "1.2 TB 可用".into(),
      capacity_percent: 82,
      library_bindings: vec!["商业摄影资产库".into()],
      badges: vec!["只读".into(), "移动设备".into()],
      risk_tags: vec!["空间风险".into()],
      auth_status: "无需鉴权".into(),
      auth_tone: "info".into(),
      notes: "现场素材源盘".into(),
      detail: StorageNodeDetail::Local { root_path: "E:\\shooting-card".into() },
    },
    StorageNodeRecord {
      id: "node-nas-main".into(),
      name: "影像 NAS 01".into(),
      node_type: "NAS/SMB".into(),
      address: "\\\\192.168.10.20\\media".into(),
      mount_mode: "可写".into(),
      enabled: true,
      scan_status: "等待队列".into(),
      scan_tone: "info".into(),
      last_scan_at: "2 分钟前".into(),
      heartbeat_policy: "每日（深夜）".into(),
      next_heartbeat_at: "今晚 02:00".into(),
      last_heartbeat_result: "上次成功".into(),
      heartbeat_failures: 0,
      capacity_summary: "已用 48% · 18.9 TB 可用".into(),
      free_space_summary: "18.9 TB 可用".into(),
      capacity_percent: 48,
      library_bindings: vec!["商业摄影资产库".into(), "视频工作流资产库".into()],
      badges: vec!["可写".into(), "SMB".into()],
      risk_tags: vec![],
      auth_status: "鉴权正常".into(),
      auth_tone: "success".into(),
      notes: "影像主 NAS".into(),
      detail: StorageNodeDetail::Nas { protocol: "SMB".into(), host: "192.168.10.20".into(), share_name: "media".into(), username: "mare-sync".into(), password_hint: "已保存，最近更新于 3 天前".into() },
    },
    StorageNodeRecord {
      id: "node-nas-backup".into(),
      name: "影像 NAS 备份柜".into(),
      node_type: "NAS/SMB".into(),
      address: "\\\\192.168.10.36\\backup_media".into(),
      mount_mode: "只读".into(),
      enabled: true,
      scan_status: "最近扫描成功".into(),
      scan_tone: "success".into(),
      last_scan_at: "昨天 23:18".into(),
      heartbeat_policy: "每周（深夜）".into(),
      next_heartbeat_at: "周六 02:00".into(),
      last_heartbeat_result: "上次成功".into(),
      heartbeat_failures: 0,
      capacity_summary: "已用 71% · 9.6 TB 可用".into(),
      free_space_summary: "9.6 TB 可用".into(),
      capacity_percent: 71,
      library_bindings: vec!["家庭照片资产库".into()],
      badges: vec!["只读".into(), "SMB".into()],
      risk_tags: vec![],
      auth_status: "鉴权正常".into(),
      auth_tone: "success".into(),
      notes: "历史归档备份节点".into(),
      detail: StorageNodeDetail::Nas {
        protocol: "SMB".into(),
        host: "192.168.10.36".into(),
        share_name: "backup_media".into(),
        username: "mare-archive".into(),
        password_hint: "已保存，最近更新于 7 天前".into(),
      },
    },
    StorageNodeRecord {
      id: "node-cloud-115".into(),
      name: "115 云归档".into(),
      node_type: "网盘".into(),
      address: "/MareArchive".into(),
      mount_mode: "可写".into(),
      enabled: true,
      scan_status: "最近扫描失败".into(),
      scan_tone: "critical".into(),
      last_scan_at: "今天 07:40".into(),
      heartbeat_policy: "每周（深夜）".into(),
      next_heartbeat_at: "周六 02:00".into(),
      last_heartbeat_result: "连续 2 次失败".into(),
      heartbeat_failures: 2,
      capacity_summary: "远端容量正常 · 约 37% 已使用".into(),
      free_space_summary: "远端容量正常".into(),
      capacity_percent: 37,
      library_bindings: vec!["商业摄影资产库".into()],
      badges: vec!["115".into(), "可写".into()],
      risk_tags: vec!["扫描失败".into(), "鉴权异常".into()],
      auth_status: "令牌 48 小时内过期".into(),
      auth_tone: "warning".into(),
      notes: "云归档节点".into(),
      detail: StorageNodeDetail::Cloud {
        vendor: "115".into(),
        account_alias: "mare-archive".into(),
        mount_directory: "/MareArchive".into(),
        access_method: "填入 Token".into(),
        qr_channel: None,
        token_status: "48 小时内过期".into(),
      },
    },
    StorageNodeRecord {
      id: "node-cloud-project".into(),
      name: "115 项目交换区".into(),
      node_type: "网盘".into(),
      address: "/ProjectExchange".into(),
      mount_mode: "可写".into(),
      enabled: true,
      scan_status: "最近扫描成功".into(),
      scan_tone: "success".into(),
      last_scan_at: "今天 08:20".into(),
      heartbeat_policy: "每日（深夜）".into(),
      next_heartbeat_at: "今晚 02:00".into(),
      last_heartbeat_result: "上次成功".into(),
      heartbeat_failures: 0,
      capacity_summary: "远端容量正常 · 约 22% 已使用".into(),
      free_space_summary: "远端容量正常".into(),
      capacity_percent: 22,
      library_bindings: vec!["视频工作流资产库".into()],
      badges: vec!["115".into(), "可写".into()],
      risk_tags: vec![],
      auth_status: "鉴权正常".into(),
      auth_tone: "success".into(),
      notes: "项目交换临时空间".into(),
      detail: StorageNodeDetail::Cloud {
        vendor: "115".into(),
        account_alias: "mare-exchange".into(),
        mount_directory: "/ProjectExchange".into(),
        access_method: "扫码登录获取 Token".into(),
        qr_channel: Some("微信小程序".into()),
        token_status: "已配置".into(),
      },
    },
  ]
}

fn seed_histories() -> HashMap<String, Vec<StorageScanHistoryItem>> {
  HashMap::from([
    (
      "node-cloud-115".into(),
      vec![
        StorageScanHistoryItem { id: "history-cloud-1".into(), started_at: "2026-03-31 02:00".into(), finished_at: "2026-03-31 02:18".into(), status: "成功".into(), summary: "新增 218 项，变更 12 项，未发现异常。".into(), trigger: "计划扫描".into() },
        StorageScanHistoryItem { id: "history-cloud-2".into(), started_at: "2026-03-30 02:00".into(), finished_at: "2026-03-30 02:06".into(), status: "失败".into(), summary: "远端目录读取超时，已写入异常中心。".into(), trigger: "计划扫描".into() },
      ],
    ),
    (
      "node-nas-main".into(),
      vec![StorageScanHistoryItem { id: "history-nas-1".into(), started_at: "2026-03-31 02:00".into(), finished_at: "2026-03-31 02:18".into(), status: "成功".into(), summary: "新增 218 项，变更 12 项，未发现异常。".into(), trigger: "计划扫描".into() }],
    ),
  ])
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
  tauri::Builder::default()
    .setup(|app| {
      if cfg!(debug_assertions) {
        app.handle().plugin(
          tauri_plugin_log::Builder::default()
            .level(log::LevelFilter::Info)
            .build(),
        )?;
      }
      Ok(())
    })
    .invoke_handler(tauri::generate_handler![
      storage_nodes_load_dashboard,
      storage_nodes_save_node,
      storage_nodes_run_scan,
      storage_nodes_run_connection_test,
      storage_nodes_update_heartbeat,
      storage_nodes_save_credentials,
      storage_nodes_update_enabled,
      storage_nodes_delete_node,
      storage_nodes_load_scan_history
    ])
    .run(tauri::generate_context!())
    .expect("error while running tauri application");
}
