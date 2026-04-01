use std::{
  collections::HashMap,
  fs,
  path::{Path, PathBuf},
};

use rfd::FileDialog;
use rusqlite::{params, Connection};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use tauri::{AppHandle, Manager};

const DB_FILE_NAME: &str = "storage_nodes_mock.sqlite3";

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MessageResponse {
  message: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct FolderBrowseResponse {
  path: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StorageNodesDashboard {
  mount_folders: Vec<MountFolderRecord>,
  nas_nodes: Vec<NasRecord>,
  cloud_nodes: Vec<CloudRecord>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MountFolderRecord {
  id: String,
  name: String,
  library_id: String,
  library_name: String,
  folder_type: String,
  source_ref_id: Option<String>,
  source_name: Option<String>,
  address: String,
  mount_mode: String,
  enabled: bool,
  scan_status: String,
  scan_tone: String,
  last_scan_at: String,
  heartbeat_policy: String,
  next_heartbeat_at: String,
  capacity_summary: String,
  free_space_summary: String,
  capacity_percent: i64,
  risk_tags: Vec<String>,
  badges: Vec<String>,
  auth_status: String,
  auth_tone: String,
  notes: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct NasRecord {
  id: String,
  name: String,
  address: String,
  username: String,
  password_hint: String,
  last_test_at: Option<String>,
  status: String,
  tone: String,
  notes: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct CloudRecord {
  id: String,
  name: String,
  vendor: String,
  access_method: String,
  qr_channel: Option<String>,
  account_alias: String,
  mount_directory: String,
  token_status: String,
  last_test_at: Option<String>,
  status: String,
  tone: String,
  notes: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MountFolderDraft {
  id: Option<String>,
  name: String,
  library_id: String,
  folder_type: String,
  mount_mode: String,
  heartbeat_policy: String,
  local_path: String,
  nas_id: String,
  cloud_id: String,
  target_folder: String,
  notes: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct NasDraft {
  id: Option<String>,
  name: String,
  address: String,
  username: String,
  password: String,
  notes: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct CloudDraft {
  id: Option<String>,
  name: String,
  vendor: String,
  access_method: String,
  qr_channel: String,
  account_alias: String,
  mount_directory: String,
  token: String,
  notes: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StorageConnectionTestResult {
  id: String,
  name: String,
  overall_tone: String,
  summary: String,
  checks: Vec<ConnectionCheck>,
  suggestion: Option<String>,
  tested_at: String,
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
struct ConnectionTestResponse {
  message: String,
  results: Vec<StorageConnectionTestResult>,
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
struct MountHistoryResponse {
  id: String,
  items: Vec<StorageScanHistoryItem>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct IdPayload {
  id: String,
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

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
struct HistoryStore {
  items: HashMap<String, Vec<StorageScanHistoryItem>>,
}

#[tauri::command]
fn storage_nodes_load_dashboard(app: AppHandle) -> Result<StorageNodesDashboard, String> {
  let conn = open_database(&app)?;
  read_store::<StorageNodesDashboard>(&conn, "dashboard")
}

#[tauri::command]
fn storage_nodes_save_mount_folder(app: AppHandle, draft: MountFolderDraft) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut dashboard = read_store::<StorageNodesDashboard>(&conn, "dashboard")?;
  let record = mount_draft_to_record(&dashboard, draft);

  if let Some(existing_id) = dashboard.mount_folders.iter().position(|item| item.id == record.id) {
    dashboard.mount_folders[existing_id] = record.clone();
  } else {
    dashboard.mount_folders.insert(0, record);
  }

  write_store(&conn, "dashboard", &dashboard)?;
  Ok(MessageResponse { message: "挂载文件夹已保存".into() })
}

#[tauri::command]
fn storage_nodes_save_nas_node(app: AppHandle, draft: NasDraft) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut dashboard = read_store::<StorageNodesDashboard>(&conn, "dashboard")?;
  let existing_id = draft.id.clone().unwrap_or_default();
  let record = NasRecord {
    id: draft.id.unwrap_or_else(|| format!("nas-{}", random_id())),
    name: draft.name,
    address: draft.address,
    username: draft.username,
    password_hint: if draft.password.is_empty() { "未更新".into() } else { "刚刚更新".into() },
    last_test_at: dashboard.nas_nodes.iter().find(|item| item.id == existing_id).and_then(|item| item.last_test_at.clone()),
    status: "鉴权正常".into(),
    tone: "success".into(),
    notes: draft.notes,
  };

  if let Some(index) = dashboard.nas_nodes.iter().position(|item| item.id == record.id) {
    dashboard.nas_nodes[index] = record;
  } else {
    dashboard.nas_nodes.insert(0, record);
  }

  write_store(&conn, "dashboard", &dashboard)?;
  Ok(MessageResponse { message: "NAS 已保存".into() })
}

#[tauri::command]
fn storage_nodes_save_cloud_node(app: AppHandle, draft: CloudDraft) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut dashboard = read_store::<StorageNodesDashboard>(&conn, "dashboard")?;
  let existing_id = draft.id.clone().unwrap_or_default();
  let record = CloudRecord {
    id: draft.id.unwrap_or_else(|| format!("cloud-{}", random_id())),
    name: draft.name,
    vendor: draft.vendor,
    access_method: draft.access_method.clone(),
    qr_channel: if draft.access_method == "扫码登录获取 Token" { Some(draft.qr_channel) } else { None },
    account_alias: draft.account_alias,
    mount_directory: draft.mount_directory,
    token_status: if draft.token.is_empty() { "未配置".into() } else { "已配置".into() },
    last_test_at: dashboard.cloud_nodes.iter().find(|item| item.id == existing_id).and_then(|item| item.last_test_at.clone()),
    status: if draft.token.is_empty() { "待鉴权".into() } else { "鉴权正常".into() },
    tone: if draft.token.is_empty() { "warning".into() } else { "success".into() },
    notes: draft.notes,
  };

  if let Some(index) = dashboard.cloud_nodes.iter().position(|item| item.id == record.id) {
    dashboard.cloud_nodes[index] = record;
  } else {
    dashboard.cloud_nodes.insert(0, record);
  }

  write_store(&conn, "dashboard", &dashboard)?;
  Ok(MessageResponse { message: "网盘已保存".into() })
}

#[tauri::command]
fn storage_nodes_run_mount_scan(app: AppHandle, payload: IdsPayload) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut dashboard = read_store::<StorageNodesDashboard>(&conn, "dashboard")?;
  let mut histories = read_store::<HistoryStore>(&conn, "histories")?;

  for mount in dashboard.mount_folders.iter_mut().filter(|item| payload.ids.contains(&item.id)) {
    mount.scan_status = "扫描中".into();
    mount.scan_tone = "warning".into();
    mount.last_scan_at = "正在执行".into();
    histories.items.entry(mount.id.clone()).or_default().insert(
      0,
      StorageScanHistoryItem {
        id: format!("history-{}", random_id()),
        started_at: "刚刚".into(),
        finished_at: "进行中".into(),
        status: "进行中".into(),
        summary: format!("{} 扫描任务已创建。", mount.name),
        trigger: if payload.ids.len() > 1 { "批量扫描".into() } else { "手动扫描".into() },
      },
    );
  }

  write_store(&conn, "dashboard", &dashboard)?;
  write_store(&conn, "histories", &histories)?;

  Ok(MessageResponse {
    message: format!("已为 {} 个挂载文件夹创建扫描任务", payload.ids.len()),
  })
}

#[tauri::command]
fn storage_nodes_run_mount_connection_test(
  app: AppHandle,
  payload: IdsPayload,
) -> Result<ConnectionTestResponse, String> {
  let conn = open_database(&app)?;
  let dashboard = read_store::<StorageNodesDashboard>(&conn, "dashboard")?;
  let results = payload
    .ids
    .iter()
    .filter_map(|id| dashboard.mount_folders.iter().find(|item| &item.id == id))
    .map(|mount| StorageConnectionTestResult {
      id: mount.id.clone(),
      name: mount.name.clone(),
      overall_tone: if mount.risk_tags.is_empty() { "success".into() } else { "warning".into() },
      summary: if mount.risk_tags.is_empty() {
        "挂载目录可达且当前配置可继续使用。".into()
      } else {
        "当前挂载目录可达，但建议先处理风险提示。".into()
      },
      checks: vec![
        ConnectionCheck { label: "可达性".into(), status: "success".into(), detail: format!("{} 可访问。", mount.address) },
        ConnectionCheck { label: "鉴权状态".into(), status: mount.auth_tone.clone(), detail: mount.auth_status.clone() },
        ConnectionCheck { label: "读权限".into(), status: "success".into(), detail: "可读取挂载目录。".into() },
        ConnectionCheck { label: "写权限".into(), status: if mount.mount_mode == "只读" { "warning".into() } else { "success".into() }, detail: if mount.mount_mode == "只读" { "当前为只读挂载。".into() } else { "可写入挂载目录。".into() } },
        ConnectionCheck { label: "目标目录可访问".into(), status: "success".into(), detail: "目录检查通过。".into() },
      ],
      suggestion: Some(if mount.risk_tags.is_empty() { "可立即执行扫描".into() } else { "检查配置".into() }),
      tested_at: "刚刚".into(),
    })
    .collect::<Vec<_>>();

  Ok(ConnectionTestResponse {
    message: if payload.ids.len() > 1 {
      format!("已完成 {} 个挂载文件夹的连接测试", payload.ids.len())
    } else {
      "连接测试已完成".into()
    },
    results,
  })
}

#[tauri::command]
fn storage_nodes_run_nas_connection_test(
  app: AppHandle,
  payload: IdsPayload,
) -> Result<ConnectionTestResponse, String> {
  let conn = open_database(&app)?;
  let mut dashboard = read_store::<StorageNodesDashboard>(&conn, "dashboard")?;
  let results = payload
    .ids
    .iter()
    .filter_map(|id| dashboard.nas_nodes.iter().find(|item| &item.id == id))
    .enumerate()
    .map(|(index, item)| StorageConnectionTestResult {
      id: item.id.clone(),
      name: item.name.clone(),
      overall_tone: if index % 2 == 0 { "success".into() } else { "warning".into() },
      summary: if index % 2 == 0 {
        "NAS 连接测试通过，可继续使用当前配置。".into()
      } else {
        "NAS 可达，但账号密码校验未通过。".into()
      },
      checks: vec![
        ConnectionCheck { label: "可达性".into(), status: "success".into(), detail: format!("{} 可达。", item.address) },
        ConnectionCheck { label: "鉴权状态".into(), status: if index % 2 == 0 { "success".into() } else { "warning".into() }, detail: if index % 2 == 0 { "账号密码验证通过。".into() } else { "账号密码需要重新确认。".into() } },
      ],
      suggestion: Some(if index % 2 == 0 { "可继续挂载".into() } else { "检查账号密码".into() }),
      tested_at: "刚刚".into(),
    })
    .collect::<Vec<_>>();

  for nas in dashboard.nas_nodes.iter_mut().filter(|item| payload.ids.contains(&item.id)) {
    let is_success = results.iter().find(|result| result.id == nas.id).map(|result| result.overall_tone == "success").unwrap_or(false);
    nas.status = if is_success { "鉴权正常".into() } else { "鉴权异常".into() };
    nas.tone = if is_success { "success".into() } else { "warning".into() };
    nas.last_test_at = Some("刚刚".into());
  }

  write_store(&conn, "dashboard", &dashboard)?;
  Ok(ConnectionTestResponse {
    message: if payload.ids.len() > 1 { format!("已完成 {} 个 NAS 的连接测试", payload.ids.len()) } else { "连接测试已完成".into() },
    results,
  })
}

#[tauri::command]
fn storage_nodes_run_cloud_connection_test(
  app: AppHandle,
  payload: IdsPayload,
) -> Result<ConnectionTestResponse, String> {
  let conn = open_database(&app)?;
  let mut dashboard = read_store::<StorageNodesDashboard>(&conn, "dashboard")?;
  let results = payload
    .ids
    .iter()
    .filter_map(|id| dashboard.cloud_nodes.iter().find(|item| &item.id == id))
    .enumerate()
    .map(|(index, item)| StorageConnectionTestResult {
      id: item.id.clone(),
      name: item.name.clone(),
      overall_tone: if index % 2 == 0 { "success".into() } else { "warning".into() },
      summary: if index % 2 == 0 {
        "网盘连接测试通过，Token 当前可继续使用。".into()
      } else {
        "网盘可达，但 Token 需要重新确认。".into()
      },
      checks: vec![
        ConnectionCheck { label: "可达性".into(), status: "success".into(), detail: format!("{} 可访问。", item.mount_directory) },
        ConnectionCheck { label: "鉴权状态".into(), status: if index % 2 == 0 { "success".into() } else { "warning".into() }, detail: if index % 2 == 0 { "Token 验证通过。".into() } else { "Token 已失效或即将过期。".into() } },
      ],
      suggestion: Some(if index % 2 == 0 { "可继续挂载".into() } else { "重新获取 Token".into() }),
      tested_at: "刚刚".into(),
    })
    .collect::<Vec<_>>();

  for cloud in dashboard.cloud_nodes.iter_mut().filter(|item| payload.ids.contains(&item.id)) {
    let is_success = results.iter().find(|result| result.id == cloud.id).map(|result| result.overall_tone == "success").unwrap_or(false);
    cloud.status = if is_success { "鉴权正常".into() } else { "鉴权异常".into() };
    cloud.tone = if is_success { "success".into() } else { "warning".into() };
    cloud.last_test_at = Some("刚刚".into());
  }

  write_store(&conn, "dashboard", &dashboard)?;
  Ok(ConnectionTestResponse {
    message: if payload.ids.len() > 1 { format!("已完成 {} 个网盘的连接测试", payload.ids.len()) } else { "连接测试已完成".into() },
    results,
  })
}

#[tauri::command]
fn storage_nodes_update_mount_heartbeat(
  app: AppHandle,
  payload: UpdateHeartbeatPayload,
) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut dashboard = read_store::<StorageNodesDashboard>(&conn, "dashboard")?;

  for mount in dashboard.mount_folders.iter_mut().filter(|item| payload.ids.contains(&item.id)) {
    mount.heartbeat_policy = payload.heartbeat_policy.clone();
    mount.next_heartbeat_at = next_heartbeat_at(&payload.heartbeat_policy).into();
  }

  write_store(&conn, "dashboard", &dashboard)?;
  Ok(MessageResponse { message: "心跳策略已更新".into() })
}

#[tauri::command]
fn storage_nodes_load_mount_scan_history(app: AppHandle, payload: IdPayload) -> Result<MountHistoryResponse, String> {
  let conn = open_database(&app)?;
  let histories = read_store::<HistoryStore>(&conn, "histories")?;
  Ok(MountHistoryResponse {
    id: payload.id.clone(),
    items: histories.items.get(&payload.id).cloned().unwrap_or_default(),
  })
}

#[tauri::command]
fn storage_nodes_delete_mount_folder(app: AppHandle, payload: IdPayload) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut dashboard = read_store::<StorageNodesDashboard>(&conn, "dashboard")?;
  let mut histories = read_store::<HistoryStore>(&conn, "histories")?;
  dashboard.mount_folders.retain(|item| item.id != payload.id);
  histories.items.remove(&payload.id);
  write_store(&conn, "dashboard", &dashboard)?;
  write_store(&conn, "histories", &histories)?;
  Ok(MessageResponse { message: "挂载文件夹已删除".into() })
}

#[tauri::command]
fn storage_nodes_delete_nas_node(app: AppHandle, payload: IdPayload) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut dashboard = read_store::<StorageNodesDashboard>(&conn, "dashboard")?;
  dashboard.nas_nodes.retain(|item| item.id != payload.id);
  dashboard.mount_folders.retain(|item| item.source_ref_id.as_deref() != Some(&payload.id));
  write_store(&conn, "dashboard", &dashboard)?;
  Ok(MessageResponse { message: "NAS 已删除".into() })
}

#[tauri::command]
fn storage_nodes_delete_cloud_node(app: AppHandle, payload: IdPayload) -> Result<MessageResponse, String> {
  let conn = open_database(&app)?;
  let mut dashboard = read_store::<StorageNodesDashboard>(&conn, "dashboard")?;
  dashboard.cloud_nodes.retain(|item| item.id != payload.id);
  dashboard.mount_folders.retain(|item| item.source_ref_id.as_deref() != Some(&payload.id));
  write_store(&conn, "dashboard", &dashboard)?;
  Ok(MessageResponse { message: "网盘已删除".into() })
}

#[tauri::command]
fn storage_nodes_browse_local_folder() -> Result<FolderBrowseResponse, String> {
  let path = FileDialog::new().pick_folder().map(|path| path.display().to_string());
  Ok(FolderBrowseResponse { path })
}

fn open_database(app: &AppHandle) -> Result<Connection, String> {
  let path = db_path(app)?;
  if let Some(parent) = path.parent() {
    fs::create_dir_all(parent).map_err(|error| error.to_string())?;
  }
  let conn = Connection::open(path).map_err(|error| error.to_string())?;
  conn
    .execute_batch("CREATE TABLE IF NOT EXISTS mock_store (key TEXT PRIMARY KEY, payload TEXT NOT NULL);")
    .map_err(|error| error.to_string())?;
  ensure_seed(&conn)?;
  Ok(conn)
}

fn db_path(app: &AppHandle) -> Result<PathBuf, String> {
  let base = app.path().app_data_dir().map_err(|error| error.to_string())?;
  Ok(base.join(Path::new(DB_FILE_NAME)))
}

fn ensure_seed(conn: &Connection) -> Result<(), String> {
  if read_store::<StorageNodesDashboard>(conn, "dashboard").is_err() {
    write_store(conn, "dashboard", &seed_dashboard())?;
  }
  if read_store::<HistoryStore>(conn, "histories").is_err() {
    write_store(conn, "histories", &seed_histories())?;
  }
  Ok(())
}

fn read_store<T: DeserializeOwned>(conn: &Connection, key: &str) -> Result<T, String> {
  let payload: String = conn
    .query_row("SELECT payload FROM mock_store WHERE key = ?1", params![key], |row| row.get(0))
    .map_err(|error| error.to_string())?;
  serde_json::from_str(&payload).map_err(|error| error.to_string())
}

fn write_store<T: Serialize>(conn: &Connection, key: &str, value: &T) -> Result<(), String> {
  let payload = serde_json::to_string(value).map_err(|error| error.to_string())?;
  conn
    .execute(
      "INSERT INTO mock_store (key, payload) VALUES (?1, ?2)
       ON CONFLICT(key) DO UPDATE SET payload = excluded.payload",
      params![key, payload],
    )
    .map_err(|error| error.to_string())?;
  Ok(())
}

fn mount_draft_to_record(dashboard: &StorageNodesDashboard, draft: MountFolderDraft) -> MountFolderRecord {
  let mount_mode = draft.mount_mode.clone();
  let heartbeat_policy = draft.heartbeat_policy.clone();
  let library_name = match draft.library_id.as_str() {
    "photo" => "商业摄影资产库",
    "video" => "视频工作流资产库",
    "family" => "家庭照片资产库",
    _ => "未命名资产库",
  }
  .to_string();

  match draft.folder_type.as_str() {
    "本地" => MountFolderRecord {
      id: draft.id.unwrap_or_else(|| format!("mount-{}", random_id())),
      name: draft.name,
      library_id: draft.library_id,
      library_name,
      folder_type: "本地".into(),
      source_ref_id: None,
      source_name: None,
      address: draft.local_path,
      mount_mode: mount_mode.clone(),
      enabled: true,
      scan_status: "未扫描".into(),
      scan_tone: "info".into(),
      last_scan_at: "未扫描".into(),
      heartbeat_policy: heartbeat_policy.clone(),
      next_heartbeat_at: next_heartbeat_at(&heartbeat_policy).into(),
      capacity_summary: "待首次检测".into(),
      free_space_summary: "待首次检测".into(),
      capacity_percent: 0,
      risk_tags: vec![],
      badges: vec!["本地".into(), mount_mode],
      auth_status: "无需鉴权".into(),
      auth_tone: "info".into(),
      notes: draft.notes,
    },
    "NAS" => {
      let nas = dashboard.nas_nodes.iter().find(|item| item.id == draft.nas_id);
      let relative = draft.target_folder.trim().trim_matches(['\\', '/']).to_string();
      MountFolderRecord {
        id: draft.id.unwrap_or_else(|| format!("mount-{}", random_id())),
        name: draft.name,
        library_id: draft.library_id,
        library_name,
        folder_type: "NAS".into(),
        source_ref_id: nas.map(|item| item.id.clone()),
        source_name: nas.map(|item| item.name.clone()),
        address: if relative.is_empty() {
          nas.map(|item| item.address.clone()).unwrap_or_default()
        } else {
          format!("{}\\{}", nas.map(|item| item.address.clone()).unwrap_or_default(), relative)
        },
        mount_mode: mount_mode.clone(),
        enabled: true,
        scan_status: "未扫描".into(),
        scan_tone: "info".into(),
        last_scan_at: "未扫描".into(),
        heartbeat_policy: heartbeat_policy.clone(),
        next_heartbeat_at: next_heartbeat_at(&heartbeat_policy).into(),
        capacity_summary: "待首次检测".into(),
        free_space_summary: "待首次检测".into(),
        capacity_percent: 0,
        risk_tags: vec![],
        badges: vec!["SMB".into(), mount_mode],
        auth_status: nas.map(|item| item.status.clone()).unwrap_or_else(|| "待鉴权".into()),
        auth_tone: nas.map(|item| item.tone.clone()).unwrap_or_else(|| "warning".into()),
        notes: draft.notes,
      }
    }
    _ => {
      let cloud = dashboard.cloud_nodes.iter().find(|item| item.id == draft.cloud_id);
      let relative = draft.target_folder.trim().trim_matches(['\\', '/']).to_string();
      MountFolderRecord {
        id: draft.id.unwrap_or_else(|| format!("mount-{}", random_id())),
        name: draft.name,
        library_id: draft.library_id,
        library_name,
        folder_type: "网盘".into(),
        source_ref_id: cloud.map(|item| item.id.clone()),
        source_name: cloud.map(|item| item.name.clone()),
        address: if relative.is_empty() {
          cloud.map(|item| item.mount_directory.clone()).unwrap_or_default()
        } else {
          format!("{}/{}", cloud.map(|item| item.mount_directory.clone()).unwrap_or_default(), relative).replace("//", "/")
        },
        mount_mode: mount_mode.clone(),
        enabled: true,
        scan_status: "未扫描".into(),
        scan_tone: "info".into(),
        last_scan_at: "未扫描".into(),
        heartbeat_policy: heartbeat_policy.clone(),
        next_heartbeat_at: next_heartbeat_at(&heartbeat_policy).into(),
        capacity_summary: "远端容量待检测".into(),
        free_space_summary: "待首次检测".into(),
        capacity_percent: 0,
        risk_tags: vec![],
        badges: vec![cloud.map(|item| item.vendor.clone()).unwrap_or_else(|| "网盘".into()), mount_mode],
        auth_status: cloud.map(|item| item.status.clone()).unwrap_or_else(|| "待鉴权".into()),
        auth_tone: cloud.map(|item| item.tone.clone()).unwrap_or_else(|| "warning".into()),
        notes: draft.notes,
      }
    }
  }
}

fn seed_dashboard() -> StorageNodesDashboard {
  let mut mount_folders = vec![
      MountFolderRecord {
        id: "mount-local-main".into(),
        name: "商业摄影原片库".into(),
        library_id: "photo".into(),
        library_name: "商业摄影资产库".into(),
        folder_type: "本地".into(),
        source_ref_id: None,
        source_name: None,
        address: "D:\\Mare\\Assets\\PhotoRaw".into(),
        mount_mode: "可写".into(),
        enabled: true,
        scan_status: "最近扫描成功".into(),
        scan_tone: "success".into(),
        last_scan_at: "今天 09:12".into(),
        heartbeat_policy: "从不".into(),
        next_heartbeat_at: "—".into(),
        capacity_summary: "已用 64% · 3.4 TB 可用".into(),
        free_space_summary: "3.4 TB 可用".into(),
        capacity_percent: 64,
        risk_tags: vec![],
        badges: vec!["本地".into(), "可写".into()],
        auth_status: "无需鉴权".into(),
        auth_tone: "info".into(),
        notes: "商业摄影本地主挂载目录".into(),
      },
      MountFolderRecord {
        id: "mount-nas-main".into(),
        name: "视频工作流 NAS 挂载".into(),
        library_id: "video".into(),
        library_name: "视频工作流资产库".into(),
        folder_type: "NAS".into(),
        source_ref_id: Some("nas-main".into()),
        source_name: Some("影像 NAS 01".into()),
        address: "\\\\192.168.10.20\\media\\video_workflow".into(),
        mount_mode: "可写".into(),
        enabled: true,
        scan_status: "等待队列".into(),
        scan_tone: "info".into(),
        last_scan_at: "2 分钟前".into(),
        heartbeat_policy: "每日（深夜）".into(),
        next_heartbeat_at: "今晚 02:00".into(),
        capacity_summary: "已用 48% · 18.9 TB 可用".into(),
        free_space_summary: "18.9 TB 可用".into(),
        capacity_percent: 48,
        risk_tags: vec![],
        badges: vec!["SMB".into(), "可写".into()],
        auth_status: "鉴权正常".into(),
        auth_tone: "success".into(),
        notes: "视频资产库主挂载目录".into(),
      },
      MountFolderRecord {
        id: "mount-cloud-archive".into(),
        name: "家庭照片网盘归档".into(),
        library_id: "family".into(),
        library_name: "家庭照片资产库".into(),
        folder_type: "网盘".into(),
        source_ref_id: Some("cloud-archive".into()),
        source_name: Some("115 云归档".into()),
        address: "/MareArchive/family_album".into(),
        mount_mode: "可写".into(),
        enabled: true,
        scan_status: "最近扫描失败".into(),
        scan_tone: "critical".into(),
        last_scan_at: "今天 07:40".into(),
        heartbeat_policy: "每周（深夜）".into(),
        next_heartbeat_at: "周六 02:00".into(),
        capacity_summary: "远端容量正常 · 约 37% 已使用".into(),
        free_space_summary: "远端容量正常".into(),
        capacity_percent: 37,
        risk_tags: vec!["扫描失败".into(), "鉴权异常".into()],
        badges: vec!["115".into(), "可写".into()],
        auth_status: "Token 48 小时内过期".into(),
        auth_tone: "warning".into(),
        notes: "家庭照片网盘归档目录".into(),
      },
    ];

  let mut nas_nodes = vec![
      NasRecord {
        id: "nas-main".into(),
        name: "影像 NAS 01".into(),
        address: "\\\\192.168.10.20\\media".into(),
        username: "mare-sync".into(),
        password_hint: "已保存，最近更新于 3 天前".into(),
        last_test_at: Some("今天 10:12".into()),
        status: "鉴权正常".into(),
        tone: "success".into(),
        notes: "主 NAS".into(),
      },
      NasRecord {
        id: "nas-backup".into(),
        name: "影像 NAS 备份柜".into(),
        address: "\\\\192.168.10.36\\backup_media".into(),
        username: "mare-archive".into(),
        password_hint: "已保存，最近更新于 7 天前".into(),
        last_test_at: Some("昨天 22:18".into()),
        status: "鉴权正常".into(),
        tone: "success".into(),
        notes: "备份 NAS".into(),
      },
    ];

  let mut cloud_nodes = vec![
      CloudRecord {
        id: "cloud-archive".into(),
        name: "115 云归档".into(),
        vendor: "115".into(),
        access_method: "填入 Token".into(),
        qr_channel: None,
        account_alias: "mare-archive".into(),
        mount_directory: "/MareArchive".into(),
        token_status: "48 小时内过期".into(),
        last_test_at: Some("今天 08:40".into()),
        status: "鉴权异常".into(),
        tone: "warning".into(),
        notes: "云归档空间".into(),
      },
      CloudRecord {
        id: "cloud-exchange".into(),
        name: "115 项目交换区".into(),
        vendor: "115".into(),
        access_method: "扫码登录获取 Token".into(),
        qr_channel: Some("微信小程序".into()),
        account_alias: "mare-exchange".into(),
        mount_directory: "/ProjectExchange".into(),
        token_status: "已配置".into(),
        last_test_at: Some("今天 09:05".into()),
        status: "鉴权正常".into(),
        tone: "success".into(),
        notes: "项目交换空间".into(),
      },
    ];

  for index in 0..22 {
    nas_nodes.push(NasRecord {
      id: format!("nas-extra-{}", index + 1),
      name: format!("项目 NAS {}", index + 1),
      address: format!("\\\\192.168.20.{}\\share_{}", 100 + index, index + 1),
      username: format!("project-user-{}", index + 1),
      password_hint: format!("已保存，最近更新于 {} 天前", index + 1),
      last_test_at: Some(format!("昨天 {:02}:00", (index % 9) + 10)),
      status: if index % 4 == 0 { "鉴权异常".into() } else { "鉴权正常".into() },
      tone: if index % 4 == 0 { "warning".into() } else { "success".into() },
      notes: format!("项目 NAS {}", index + 1),
    });

    cloud_nodes.push(CloudRecord {
      id: format!("cloud-extra-{}", index + 1),
      name: format!("115 分区 {}", index + 1),
      vendor: "115".into(),
      access_method: if index % 2 == 0 { "填入 Token".into() } else { "扫码登录获取 Token".into() },
      qr_channel: if index % 2 == 0 { None } else if index % 3 == 0 { Some("支付宝小程序".into()) } else { Some("微信小程序".into()) },
      account_alias: format!("cloud-alias-{}", index + 1),
      mount_directory: format!("/ProjectSpace/{}", index + 1),
      token_status: if index % 5 == 0 { "即将过期".into() } else { "已配置".into() },
      last_test_at: Some(format!("今天 {:02}:30", (index % 10) + 8)),
      status: if index % 5 == 0 { "鉴权异常".into() } else { "鉴权正常".into() },
      tone: if index % 5 == 0 { "warning".into() } else { "success".into() },
      notes: format!("网盘空间 {}", index + 1),
    });

    mount_folders.push(MountFolderRecord {
      id: format!("mount-extra-{}", index + 1),
      name: format!("扩展挂载 {}", index + 1),
      library_id: if index % 3 == 0 { "photo".into() } else if index % 3 == 1 { "video".into() } else { "family".into() },
      library_name: if index % 3 == 0 { "商业摄影资产库".into() } else if index % 3 == 1 { "视频工作流资产库".into() } else { "家庭照片资产库".into() },
      folder_type: if index % 3 == 0 { "本地".into() } else if index % 3 == 1 { "NAS".into() } else { "网盘".into() },
      source_ref_id: if index % 3 == 1 { Some(format!("nas-extra-{}", (index % 22) + 1)) } else if index % 3 == 2 { Some(format!("cloud-extra-{}", (index % 22) + 1)) } else { None },
      source_name: if index % 3 == 1 { Some(format!("项目 NAS {}", (index % 22) + 1)) } else if index % 3 == 2 { Some(format!("115 分区 {}", (index % 22) + 1)) } else { None },
      address: if index % 3 == 0 {
        format!("D:\\Mare\\Assets\\Library_{}", index + 1)
      } else if index % 3 == 1 {
        format!("\\\\192.168.20.{}\\share_{}\\folder_{}", 100 + (index % 22), (index % 22) + 1, index + 1)
      } else {
        format!("/ProjectSpace/{}/folder_{}", (index % 22) + 1, index + 1)
      },
      mount_mode: if index % 4 == 0 { "只读".into() } else { "可写".into() },
      enabled: true,
      scan_status: if index % 5 == 0 { "最近扫描失败".into() } else if index % 4 == 0 { "等待队列".into() } else { "最近扫描成功".into() },
      scan_tone: if index % 5 == 0 { "critical".into() } else if index % 4 == 0 { "info".into() } else { "success".into() },
      last_scan_at: if index % 4 == 0 {
        format!("{} 分钟前", index + 1)
      } else {
        format!("今天 {:02}:{:02}", (index % 10) + 8, (index % 6) * 10)
      },
      heartbeat_policy: if index % 3 == 0 { "从不".into() } else if index % 3 == 1 { "每日（深夜）".into() } else { "每周（深夜）".into() },
      next_heartbeat_at: if index % 3 == 0 { "—".into() } else if index % 3 == 1 { "今晚 02:00".into() } else { "周六 02:00".into() },
      capacity_summary: format!("已用 {}% · {:.1} TB 可用", 35 + (index % 40), 1.5 + (index as f64 * 0.2)),
      free_space_summary: format!("{:.1} TB 可用", 1.5 + (index as f64 * 0.2)),
      capacity_percent: 35 + ((index % 40) as i64),
      risk_tags: if index % 5 == 0 { vec!["扫描失败".into()] } else { vec![] },
      badges: vec![if index % 3 == 0 { "本地".into() } else if index % 3 == 1 { "SMB".into() } else { "115".into() }, if index % 4 == 0 { "只读".into() } else { "可写".into() }],
      auth_status: if index % 3 == 0 { "无需鉴权".into() } else if index % 5 == 0 { "鉴权异常".into() } else { "鉴权正常".into() },
      auth_tone: if index % 3 == 0 { "info".into() } else if index % 5 == 0 { "warning".into() } else { "success".into() },
      notes: format!("扩展挂载记录 {}", index + 1),
    });
  }

  StorageNodesDashboard {
    mount_folders,
    nas_nodes,
    cloud_nodes,
  }
}

fn seed_histories() -> HistoryStore {
  HistoryStore {
    items: HashMap::from([(
      "mount-cloud-archive".into(),
      vec![
        StorageScanHistoryItem {
          id: "history-cloud-1".into(),
          started_at: "2026-03-31 02:00".into(),
          finished_at: "2026-03-31 02:18".into(),
          status: "成功".into(),
          summary: "新增 218 项，变更 12 项，未发现异常。".into(),
          trigger: "计划扫描".into(),
        },
        StorageScanHistoryItem {
          id: "history-cloud-2".into(),
          started_at: "2026-03-30 02:00".into(),
          finished_at: "2026-03-30 02:06".into(),
          status: "失败".into(),
          summary: "远端目录读取超时，已写入异常中心。".into(),
          trigger: "计划扫描".into(),
        },
      ],
    )]),
  }
}

fn next_heartbeat_at(policy: &str) -> &'static str {
  match policy {
    "每小时" => "1 小时后",
    "每日（深夜）" => "今晚 02:00",
    "每周（深夜）" => "周六 02:00",
    _ => "—",
  }
}

fn random_id() -> String {
  format!("{:x}", fastrand::u32(..))
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
      storage_nodes_save_mount_folder,
      storage_nodes_save_nas_node,
      storage_nodes_save_cloud_node,
      storage_nodes_run_mount_scan,
      storage_nodes_run_mount_connection_test,
      storage_nodes_run_nas_connection_test,
      storage_nodes_run_cloud_connection_test,
      storage_nodes_update_mount_heartbeat,
      storage_nodes_load_mount_scan_history,
      storage_nodes_delete_mount_folder,
      storage_nodes_delete_nas_node,
      storage_nodes_delete_cloud_node,
      storage_nodes_browse_local_folder
    ])
    .run(tauri::generate_context!())
    .expect("error while running tauri application");
}
