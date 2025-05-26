import React, { useState, useEffect } from 'react';
import { Database, HardDrive, Cloud, Play, Clock, CheckCircle, XCircle, Settings, Download, Trash2, RefreshCw, Calendar, Plus, Edit, Pause, Power } from 'lucide-react';

const PostgreSQLBackupInterface = () => {
  const [activeTab, setActiveTab] = useState('backup');
  const [backupType, setBackupType] = useState('local');
  const [isBackingUp, setIsBackingUp] = useState(false);
  const [backupHistory, setBackupHistory] = useState([
    {
      id: 1,
      name: 'daily_backup_20250526_143020',
      type: 'local',
      size: '245.8 MB',
      status: 'completed',
      timestamp: '2025-05-26 14:30:20',
      path: '/backups/postgresql/daily_backup_20250526_143020.sql'
    },
    {
      id: 2,
      name: 'weekly_backup_20250519_020015',
      type: 's3',
      size: '1.2 GB',
      status: 'completed',
      timestamp: '2025-05-19 02:00:15',
      path: 's3://my-backups/postgresql/weekly_backup_20250519_020015.sql'
    },
    {
      id: 3,
      name: 'manual_backup_20250525_091245',
      type: 'local',
      size: '189.3 MB',
      status: 'failed',
      timestamp: '2025-05-25 09:12:45',
      path: '/backups/postgresql/manual_backup_20250525_091245.sql'
    }
  ]);

  const [dbConfig, setDbConfig] = useState({
    host: 'localhost',
    port: '5432',
    database: 'myapp_production',
    username: 'postgres',
    password: ''
  });

  const [s3Config, setS3Config] = useState({
    endpoint: 'https://minio.example.com',
    accessKey: '',
    secretKey: '',
    bucket: 'postgresql-backups',
    region: 'us-east-1'
  });

  const [localConfig, setLocalConfig] = useState({
    backupPath: '/var/backups/postgresql',
    compression: true,
    retention: 30,
    verifyContent: true  // ✅ 新增字段
  });
  

  const [scheduledJobs, setScheduledJobs] = useState([
    {
      id: 1,
      name: '每日自动备份',
      type: 'local',
      schedule: '0 2 * * *',
      scheduleText: '每天凌晨 2:00',
      enabled: true,
      lastRun: '2025-05-26 02:00:15',
      nextRun: '2025-05-27 02:00:00',
      status: 'active'
    },
    {
      id: 2,
      name: '周末云端备份',
      type: 's3',
      schedule: '0 3 * * 0',
      scheduleText: '每周日凌晨 3:00',
      enabled: true,
      lastRun: '2025-05-19 03:00:22',
      nextRun: '2025-05-26 03:00:00',
      status: 'active'
    },
    {
      id: 3,
      name: '月度完整备份',
      type: 's3',
      schedule: '0 1 1 * *',
      scheduleText: '每月 1 日凌晨 1:00',
      enabled: false,
      lastRun: '2025-05-01 01:00:45',
      nextRun: '2025-06-01 01:00:00',
      status: 'paused'
    }
  ]);

  const [showJobModal, setShowJobModal] = useState(false);
  const [editingJob, setEditingJob] = useState(null);
  const [newJob, setNewJob] = useState({
    name: '',
    type: 'local',
    schedule: '',
    scheduleText: '',
    enabled: true
  });

  const handleBackup = async () => {
    setIsBackingUp(true);
    
    // 模拟备份过程
    setTimeout(() => {
      const newBackup = {
        id: Date.now(),
        name: `manual_backup_${new Date().toISOString().replace(/[:.]/g, '').slice(0, -5)}`,
        type: backupType,
        size: `${Math.floor(Math.random() * 500 + 100)}.${Math.floor(Math.random() * 9)} MB`,
        status: 'completed',
        timestamp: new Date().toLocaleString('zh-CN'),
        path: backupType === 'local' 
          ? `/backups/postgresql/manual_backup_${Date.now()}.sql`
          : `s3://${s3Config.bucket}/manual_backup_${Date.now()}.sql`
      };
      
      setBackupHistory(prev => [newBackup, ...prev]);
      setIsBackingUp(false);
    }, 3000);
  };

  const deleteBackup = (id) => {
    setBackupHistory(prev => prev.filter(backup => backup.id !== id));
  };

  const toggleJob = (id) => {
    setScheduledJobs(prev => 
      prev.map(job => 
        job.id === id 
          ? { ...job, enabled: !job.enabled, status: job.enabled ? 'paused' : 'active' }
          : job
      )
    );
  };

  const deleteJob = (id) => {
    setScheduledJobs(prev => prev.filter(job => job.id !== id));
  };

  const openJobModal = (job = null) => {
    if (job) {
      setEditingJob(job);
      setNewJob({
        name: job.name,
        type: job.type,
        schedule: job.schedule,
        scheduleText: job.scheduleText,
        enabled: job.enabled
      });
    } else {
      setEditingJob(null);
      setNewJob({
        name: '',
        type: 'local',
        schedule: '',
        scheduleText: '',
        enabled: true
      });
    }
    setShowJobModal(true);
  };

  const saveConfig = async () => {
    try {
      const res = await fetch('/api/v1/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          local: localConfig,
          s3: s3Config,
          database: dbConfig
        })
      });
  
      if (res.ok) {
        alert('配置保存成功 ✅');
      } else {
        const data = await res.json();
        alert(`保存失败 ❌：${data.error || '未知错误'}`);
      }
    } catch (err) {
      alert('保存失败 ❌：网络错误或服务器未响应');
      console.error(err);
    }
  };
  
  
  const saveJob = () => {
    if (editingJob) {
      setScheduledJobs(prev =>
        prev.map(job =>
          job.id === editingJob.id
            ? {
                ...job,
                ...newJob,
                status: newJob.enabled ? 'active' : 'paused'
              }
            : job
        )
      );
    } else {
      const job = {
        id: Date.now(),
        ...newJob,
        lastRun: '从未运行',
        nextRun: '计算中...',
        status: newJob.enabled ? 'active' : 'paused'
      };
      setScheduledJobs(prev => [...prev, job]);
    }
    setShowJobModal(false);
  };

  const getSchedulePresets = () => [
    { label: '每小时', value: '0 * * * *', text: '每小时执行一次' },
    { label: '每天凌晨2点', value: '0 2 * * *', text: '每天凌晨 2:00' },
    { label: '每周日凌晨3点', value: '0 3 * * 0', text: '每周日凌晨 3:00' },
    { label: '每月1日凌晨1点', value: '0 1 1 * *', text: '每月 1 日凌晨 1:00' },
    { label: '工作日凌晨2点', value: '0 2 * * 1-5', text: '周一到周五凌晨 2:00' }
  ];

  const getStatusIcon = (status) => {
    switch (status) {
      case 'completed':
        return <CheckCircle className="w-4 h-4 text-green-500" />;
      case 'failed':
        return <XCircle className="w-4 h-4 text-red-500" />;
      default:
        return <Clock className="w-4 h-4 text-yellow-500" />;
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-900 via-blue-900 to-slate-900">
      <div className="container mx-auto px-6 py-8">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-2">
            <div className="p-2 bg-blue-600 rounded-lg">
              <Database className="w-6 h-6 text-white" />
            </div>
            <h1 className="text-3xl font-bold text-white">PostgreSQL 备份管理</h1>
          </div>
          <p className="text-slate-300">管理您的数据库备份，支持本地存储和云端存储</p>
        </div>

        {/* Tab Navigation */}
        <div className="flex space-x-1 mb-8">
          {[
            { id: 'backup', label: '创建备份', icon: Play },
            { id: 'schedule', label: '定时备份', icon: Calendar },
            { id: 'history', label: '备份历史', icon: Clock },
            { id: 'settings', label: '配置管理', icon: Settings }
          ].map(tab => {
            const Icon = tab.icon;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-2 px-4 py-2 rounded-lg font-medium transition-all ${
                  activeTab === tab.id
                    ? 'bg-blue-600 text-white shadow-lg'
                    : 'text-slate-300 hover:text-white hover:bg-slate-700'
                }`}
              >
                <Icon className="w-4 h-4" />
                {tab.label}
              </button>
            );
          })}
        </div>

        {/* Main Content */}
        <div className="bg-white/10 backdrop-blur-lg rounded-2xl border border-white/20 overflow-hidden">
          {activeTab === 'backup' && (
            <div className="p-8">
              <h2 className="text-2xl font-bold text-white mb-6">创建新备份</h2>
              
              {/* Backup Type Selection */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-8">
                <button
                  onClick={() => setBackupType('local')}
                  className={`p-6 rounded-xl border-2 transition-all ${
                    backupType === 'local'
                      ? 'border-blue-500 bg-blue-500/20'
                      : 'border-slate-600 bg-slate-800/50 hover:border-slate-500'
                  }`}
                >
                  <div className="flex items-center gap-4">
                    <HardDrive className={`w-8 h-8 ${backupType === 'local' ? 'text-blue-400' : 'text-slate-400'}`} />
                    <div className="text-left">
                      <h3 className="text-lg font-semibold text-white">本地磁盘备份</h3>
                      <p className="text-slate-300 text-sm">备份到服务器本地磁盘</p>
                    </div>
                  </div>
                </button>

                <button
                  onClick={() => setBackupType('s3')}
                  className={`p-6 rounded-xl border-2 transition-all ${
                    backupType === 's3'
                      ? 'border-blue-500 bg-blue-500/20'
                      : 'border-slate-600 bg-slate-800/50 hover:border-slate-500'
                  }`}
                >
                  <div className="flex items-center gap-4">
                    <Cloud className={`w-8 h-8 ${backupType === 's3' ? 'text-blue-400' : 'text-slate-400'}`} />
                    <div className="text-left">
                      <h3 className="text-lg font-semibold text-white">S3/MinIO 备份</h3>
                      <p className="text-slate-300 text-sm">备份到云端对象存储</p>
                    </div>
                  </div>
                </button>
              </div>

              {/* Database Info */}
              <div className="bg-slate-800/50 rounded-xl p-6 mb-6">
                <h3 className="text-lg font-semibold text-white mb-4">数据库信息</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <p className="text-slate-300 text-sm">主机地址</p>
                    <p className="text-white font-medium">{dbConfig.host}:{dbConfig.port}</p>
                  </div>
                  <div>
                    <p className="text-slate-300 text-sm">数据库名称</p>
                    <p className="text-white font-medium">{dbConfig.database}</p>
                  </div>
                </div>
              </div>

              {/* Backup Options */}
              <div className="bg-slate-800/50 rounded-xl p-6 mb-8">
                <h3 className="text-lg font-semibold text-white mb-4">备份选项</h3>
                <div className="space-y-4">
                  <label className="flex items-center space-x-3">
                    <input type="checkbox" defaultChecked className="w-4 h-4 text-blue-600 bg-slate-700 border-slate-600 rounded focus:ring-blue-500" />
                    <span className="text-white">包含数据</span>
                  </label>
                  <label className="flex items-center space-x-3">
                    <input type="checkbox" defaultChecked className="w-4 h-4 text-blue-600 bg-slate-700 border-slate-600 rounded focus:ring-blue-500" />
                    <span className="text-white">包含表结构</span>
                  </label>
                  <label className="flex items-center space-x-3">
                    <input type="checkbox" className="w-4 h-4 text-blue-600 bg-slate-700 border-slate-600 rounded focus:ring-blue-500" />
                    <span className="text-white">压缩备份文件</span>
                  </label>
                </div>
              </div>

              {/* Backup Button */}
              <button
                onClick={handleBackup}
                disabled={isBackingUp}
                className="w-full bg-gradient-to-r from-blue-600 to-blue-700 hover:from-blue-700 hover:to-blue-800 disabled:from-slate-600 disabled:to-slate-700 text-white font-semibold py-4 px-6 rounded-xl transition-all transform hover:scale-105 disabled:scale-100 disabled:cursor-not-allowed shadow-lg"
              >
                {isBackingUp ? (
                  <div className="flex items-center justify-center gap-3">
                    <RefreshCw className="w-5 h-5 animate-spin" />
                    正在备份中...
                  </div>
                ) : (
                  <div className="flex items-center justify-center gap-3">
                    <Play className="w-5 h-5" />
                    开始备份
                  </div>
                )}
              </button>
            </div>
          )}

          {activeTab === 'schedule' && (
            <div className="p-8">
              <div className="flex justify-between items-center mb-6">
                <h2 className="text-2xl font-bold text-white">定时备份任务</h2>
                <button 
                  onClick={() => openJobModal()}
                  className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
                >
                  <Plus className="w-4 h-4" />
                  新建任务
                </button>
              </div>

              <div className="space-y-4">
                {scheduledJobs.map(job => (
                  <div key={job.id} className="bg-slate-800/50 rounded-xl p-6 border border-slate-700">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-4">
                        <div className="flex items-center gap-2">
                          {job.type === 'local' ? (
                            <HardDrive className="w-5 h-5 text-slate-400" />
                          ) : (
                            <Cloud className="w-5 h-5 text-slate-400" />
                          )}
                          <div className={`w-3 h-3 rounded-full ${
                            job.status === 'active' ? 'bg-green-500' : 'bg-yellow-500'
                          }`}></div>
                        </div>
                        <div>
                          <h3 className="text-white font-semibold">{job.name}</h3>
                          <p className="text-slate-300 text-sm">{job.scheduleText}</p>
                          <p className="text-slate-400 text-xs">Cron: {job.schedule}</p>
                        </div>
                      </div>
                      <div className="flex items-center gap-6">
                        <div className="text-right">
                          <p className="text-slate-300 text-sm">上次运行</p>
                          <p className="text-white text-sm">{job.lastRun}</p>
                        </div>
                        <div className="text-right">
                          <p className="text-slate-300 text-sm">下次运行</p>
                          <p className="text-white text-sm">{job.nextRun}</p>
                        </div>
                        <div className="flex gap-2">
                          <button
                            onClick={() => toggleJob(job.id)}
                            className={`p-2 rounded-lg transition-colors ${
                              job.enabled
                                ? 'text-yellow-400 hover:text-yellow-300 hover:bg-yellow-500/20'
                                : 'text-green-400 hover:text-green-300 hover:bg-green-500/20'
                            }`}
                            title={job.enabled ? '暂停任务' : '启用任务'}
                          >
                            {job.enabled ? <Pause className="w-4 h-4" /> : <Power className="w-4 h-4" />}
                          </button>
                          <button 
                            onClick={() => openJobModal(job)}
                            className="p-2 text-blue-400 hover:text-blue-300 hover:bg-blue-500/20 rounded-lg transition-colors"
                            title="编辑任务"
                          >
                            <Edit className="w-4 h-4" />
                          </button>
                          <button 
                            onClick={() => deleteJob(job.id)}
                            className="p-2 text-red-400 hover:text-red-300 hover:bg-red-500/20 rounded-lg transition-colors"
                            title="删除任务"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>

              {scheduledJobs.length === 0 && (
                <div className="text-center py-12">
                  <Calendar className="w-16 h-16 text-slate-600 mx-auto mb-4" />
                  <p className="text-slate-400 text-lg">暂无定时备份任务</p>
                  <p className="text-slate-500 text-sm">点击"新建任务"开始创建您的第一个定时备份</p>
                </div>
              )}
            </div>
          )}

          {activeTab === 'history' && (
            <div className="p-8">
              <div className="flex justify-between items-center mb-6">
                <h2 className="text-2xl font-bold text-white">备份历史</h2>
                <button className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors">
                  <RefreshCw className="w-4 h-4" />
                  刷新
                </button>
              </div>

              <div className="space-y-4">
                {backupHistory.map(backup => (
                  <div key={backup.id} className="bg-slate-800/50 rounded-xl p-6 border border-slate-700">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-4">
                        <div className="flex items-center gap-2">
                          {backup.type === 'local' ? (
                            <HardDrive className="w-5 h-5 text-slate-400" />
                          ) : (
                            <Cloud className="w-5 h-5 text-slate-400" />
                          )}
                          {getStatusIcon(backup.status)}
                        </div>
                        <div>
                          <h3 className="text-white font-semibold">{backup.name}</h3>
                          <p className="text-slate-300 text-sm">{backup.path}</p>
                        </div>
                      </div>
                      <div className="flex items-center gap-4">
                        <div className="text-right">
                          <p className="text-white font-medium">{backup.size}</p>
                          <p className="text-slate-300 text-sm">{backup.timestamp}</p>
                        </div>
                        <div className="flex gap-2">
                          {backup.status === 'completed' && (
                            <button className="p-2 text-blue-400 hover:text-blue-300 hover:bg-blue-500/20 rounded-lg transition-colors">
                              <Download className="w-4 h-4" />
                            </button>
                          )}
                          <button 
                            onClick={() => deleteBackup(backup.id)}
                            className="p-2 text-red-400 hover:text-red-300 hover:bg-red-500/20 rounded-lg transition-colors"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {activeTab === 'settings' && (
            <div className="p-8">
              <h2 className="text-2xl font-bold text-white mb-6">配置管理</h2>
              
              <div className="space-y-8">
                {/* Database Configuration */}
                <div className="bg-slate-800/50 rounded-xl p-6">
                  <h3 className="text-lg font-semibold text-white mb-4">数据库配置</h3>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-slate-300 mb-2">主机地址</label>
                      <input
                        type="text"
                        value={dbConfig.host}
                        onChange={(e) => setDbConfig({...dbConfig, host: e.target.value})}
                        className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-slate-300 mb-2">端口</label>
                      <input
                        type="text"
                        value={dbConfig.port}
                        onChange={(e) => setDbConfig({...dbConfig, port: e.target.value})}
                        className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-slate-300 mb-2">数据库名称</label>
                      <input
                        type="text"
                        value={dbConfig.database}
                        onChange={(e) => setDbConfig({...dbConfig, database: e.target.value})}
                        className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-slate-300 mb-2">用户名</label>
                      <input
                        type="text"
                        value={dbConfig.username}
                        onChange={(e) => setDbConfig({...dbConfig, username: e.target.value})}
                        className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      />
                    </div>
                  </div>
                </div>

                {/* S3/MinIO Configuration */}
                <div className="bg-slate-800/50 rounded-xl p-6">
                  <h3 className="text-lg font-semibold text-white mb-4">S3/MinIO 配置</h3>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-slate-300 mb-2">服务端点</label>
                      <input
                        type="text"
                        value={s3Config.endpoint}
                        onChange={(e) => setS3Config({...s3Config, endpoint: e.target.value})}
                        className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-slate-300 mb-2">存储桶名称</label>
                      <input
                        type="text"
                        value={s3Config.bucket}
                        onChange={(e) => setS3Config({...s3Config, bucket: e.target.value})}
                        className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-slate-300 mb-2">访问密钥</label>
                      <input
                        type="password"
                        value={s3Config.accessKey}
                        onChange={(e) => setS3Config({...s3Config, accessKey: e.target.value})}
                        className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-slate-300 mb-2">密钥</label>
                      <input
                        type="password"
                        value={s3Config.secretKey}
                        onChange={(e) => setS3Config({...s3Config, secretKey: e.target.value})}
                        className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      />
                    </div>
                  </div>
                </div>

                {/* Local Backup Configuration */}
                <div className="bg-slate-800/50 rounded-xl p-6">
                  <h3 className="text-lg font-semibold text-white mb-4">本地备份配置</h3>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-slate-300 mb-2">备份路径</label>
                      <input
                        type="text"
                        value={localConfig.backupPath}
                        onChange={(e) => setLocalConfig({...localConfig, backupPath: e.target.value})}
                        className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-slate-300 mb-2">保留天数</label>
                      <input
                        type="number"
                        value={localConfig.retention}
                        onChange={(e) => setLocalConfig({...localConfig, retention: parseInt(e.target.value)})}
                        className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      />
                    </div>
                    <div className="col-span-2">
                      <label className="flex items-center space-x-3 mt-4">
                        <input
                          type="checkbox"
                          checked={localConfig.verifyContent}
                          onChange={(e) => setLocalConfig({...localConfig, verifyContent: e.target.checked})}
                          className="w-4 h-4 text-blue-600 bg-slate-700 border-slate-600 rounded focus:ring-blue-500"
                        />
                        <span className="text-white">启用内容校验（检查备份中是否包含 CREATE/INSERT）</span>
                      </label>
                    </div>
                  </div>
                </div>

                <div className="flex justify-end">
                  <button
                    onClick={saveConfig}
                    className="px-6 py-3 bg-green-600 hover:bg-green-700 text-white font-semibold rounded-lg transition-colors">
                    保存配置
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default PostgreSQLBackupInterface;