<script lang="ts">
  import { onMount } from 'svelte';

  let { onLogout } = $props<{ onLogout: () => void }>();

  interface OAuthSetting {
    provider: string;
    client_id: string;
    client_secret: string;
    redirect_url: string;
    enabled: boolean;
  }

  let settings = $state<Record<string, OAuthSetting>>({
    google: { provider: 'google', client_id: '', client_secret: '', redirect_url: '', enabled: false },
    line: { provider: 'line', client_id: '', client_secret: '', redirect_url: '', enabled: false },
    meta: { provider: 'meta', client_id: '', client_secret: '', redirect_url: '', enabled: false }
  });

  let siteSettingsForm = $state({
    title: 'kifu_store',
    tab_name: 'kifu_store',
    favicon: '',
    theme_color: '#4e342e',
    external_url: ''
  });

  const computedRedirectUrl = $derived.by(() => {
    const base = siteSettingsForm.external_url || window.location.origin;
    const cleanBase = base.endsWith('/') ? base.slice(0, -1) : base;
    return (provider: string) => `${cleanBase}/api/auth/oauth/callback/${provider}`;
  });

  let activeTab = $state('site');
  let loading = $state(true);
  let saving = $state(false);

  const getM = () => (window as any).M;

  onMount(async () => {
    await Promise.all([fetchSettings(), fetchSiteSettings()]);
  });

  async function fetchSettings() {
    loading = true;
    const token = localStorage.getItem("admin_token");

    try {
      const res = await fetch("/api/admin/oauth-settings", {
        headers: { "Authorization": `Bearer ${token}` }
      });

      if (res.status === 401 || res.status === 403) {
        handleLogout();
        return;
      }

      if (!res.ok) {
        throw new Error("設定情報の取得に失敗しました。");
      }

      const list: OAuthSetting[] = await res.json();
      for (const item of list) {
        if (settings[item.provider]) {
          settings[item.provider] = item;
        }
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: `設定取得エラー: ${err.message}`, classes: 'red' });
      }
    } finally {
      loading = false;
    }
  }

  async function handleSave(provider: string) {
    saving = true;
    const token = localStorage.getItem("admin_token");
    // Populate computed redirect_url before saving
    settings[provider].redirect_url = computedRedirectUrl(provider);
    const data = settings[provider];

    try {
      const res = await fetch("/api/admin/oauth-settings", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`
        },
        body: JSON.stringify(data)
      });

      if (!res.ok) {
        const errData = await res.json();
        throw new Error(errData.error || "設定の保存に失敗しました。");
      }

      const M = getM();
      if (M) {
        M.toast({ html: `${provider.toUpperCase()} の設定を保存しました。`, classes: 'green' });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: `エラー: ${err.message}`, classes: 'red' });
      }
    } finally {
      saving = false;
    }
  }

  async function fetchSiteSettings() {
    try {
      const res = await fetch("/api/site-settings");
      if (res.ok) {
        const data = await res.json();
        siteSettingsForm.title = data.title || 'kifu_store';
        siteSettingsForm.tab_name = data.tab_name || 'kifu_store';
        siteSettingsForm.favicon = data.favicon || '';
        siteSettingsForm.theme_color = data.theme_color || '#4e342e';
        siteSettingsForm.external_url = data.external_url || '';
      }
    } catch (err: any) {
      console.error("サイト設定の取得に失敗しました", err);
    }
  }

  function handleFaviconChange(e: Event) {
    const target = e.target as HTMLInputElement;
    const file = target.files?.[0];
    if (file) {
      const reader = new FileReader();
      reader.onload = () => {
        siteSettingsForm.favicon = reader.result as string;
      };
      reader.readAsDataURL(file);
    }
  }

  async function handleSaveSiteSettings() {
    saving = true;
    const token = localStorage.getItem("admin_token");

    try {
      const res = await fetch("/api/admin/site-settings", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`
        },
        body: JSON.stringify(siteSettingsForm)
      });

      if (!res.ok) {
        const errData = await res.json();
        throw new Error(errData.error || "サイト設定の保存に失敗しました。");
      }

      const M = getM();
      if (M) {
        M.toast({ html: "サイト設定を保存しました。画面を再ロードします...", classes: 'green' });
      }
      setTimeout(() => {
        window.location.reload();
      }, 1000);
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: `エラー: ${err.message}`, classes: 'red' });
      }
    } finally {
      saving = false;
    }
  }

  function handleLogout() {
    localStorage.removeItem("admin_token");
    onLogout();
  }
</script>

<div class="row animate-fade-in" style="margin-top: 1.5rem;">
  <!-- Header section - Vogue Style -->
  <div class="col s12 d-flex justify-between align-center" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 2.5rem; flex-wrap: wrap; gap: 12px;">
    <h4 style="margin: 0; font-weight: 800; font-family: 'Shippori Mincho B1', serif; color: var(--wc-text); letter-spacing: 0.04em; text-transform: uppercase;">サイト管理設定</h4>
    <button class="logout-btn font-sans" onclick={handleLogout}>
      <i class="material-icons" style="font-size: 1.2rem;">exit_to_app</i>管理ログアウト
    </button>
  </div>

  {#if loading}
    <div class="col s12 center-align" style="margin-top: 5rem;">
      <div class="nm-spinner" style="width: 48px; height: 48px; margin: 0 auto;"></div>
    </div>
  {:else}
    <!-- Tabs Nav - Sharp Segmented tab bar -->
    <div class="col s12" style="margin-bottom: 2rem;">
      <div class="tabs-container">
        <ul class="tabs-list" style="display: flex; list-style: none; margin: 0; padding: 0; width: 100%;">
          <li style="flex: 1;">
            <button class="tab-btn w-100 {activeTab === 'site' ? 'active' : ''}" onclick={() => activeTab = 'site'}>サイト設定</button>
          </li>
          <li style="flex: 1;">
            <button class="tab-btn w-100 {activeTab === 'google' ? 'active' : ''}" onclick={() => activeTab = 'google'}>Google</button>
          </li>
          <li style="flex: 1;">
            <button class="tab-btn w-100 {activeTab === 'line' ? 'active' : ''}" onclick={() => activeTab = 'line'}>LINE</button>
          </li>
          <li style="flex: 1;">
            <button class="tab-btn w-100 {activeTab === 'meta' ? 'active' : ''}" onclick={() => activeTab = 'meta'}>Meta (Facebook)</button>
          </li>
        </ul>
      </div>
    </div>

    <!-- Active Tab Panel -->
    <div class="col s12">
      {#if activeTab === 'site'}
        <div class="nm-card admin-panel-card">
          <div class="card-content" style="padding: 2.5rem;">
            <span class="card-title font-mincho">
              <i class="material-icons title-icon">web</i> サイト基本設定
            </span>

            <div class="row" style="margin-bottom: 0;">
              <!-- Title Input -->
              <div class="input-field col s12" style="margin-bottom: 20px;">
                <input id="site_title" type="text" class="nm-input" bind:value={siteSettingsForm.title} placeholder="kifu_store" style="margin-bottom: 0;" />
                <label for="site_title" class="active font-sans">ページ右上のタイトル</label>
              </div>

              <!-- Tab Name Input -->
              <div class="input-field col s12" style="margin-bottom: 20px;">
                <input id="site_tab_name" type="text" class="nm-input" bind:value={siteSettingsForm.tab_name} placeholder="kifu_store" style="margin-bottom: 0;" />
                <label for="site_tab_name" class="active font-sans">タブ名 (ブラウザのタイトル)</label>
              </div>

              <!-- Favicon File Input -->
              <div class="col s12" style="margin-bottom: 24px;">
                <label class="font-sans" style="font-size: 0.8rem; color: var(--wc-text-muted); display: block; margin-bottom: 6px; font-weight: 600;">ファビコン (Favicon)</label>
                <div class="file-field-container">
                  <div class="file-select-btn font-sans">
                    <span>ファイル選択</span>
                    <input type="file" accept="image/*" onchange={handleFaviconChange} />
                  </div>
                  <input class="file-path-text nm-input" type="text" readonly placeholder="画像をアップロードしてください (.ico, .png, .svg など)" value={siteSettingsForm.favicon ? "ファビコン画像が選択されています" : ""} style="margin-bottom: 0;" />
                </div>
                {#if siteSettingsForm.favicon}
                  <div style="margin-top: 14px; display: flex; align-items: center; gap: 12px;">
                    <span class="font-sans" style="font-size: 0.85rem; color: var(--wc-text-muted);">プレビュー:</span>
                    <img src={siteSettingsForm.favicon} alt="Favicon Preview" style="width: 32px; height: 32px; object-fit: contain; border: 1.5px solid var(--wc-border); padding: 2px; border-radius: 0px; background: #ffffff;" />
                    <button class="delete-btn font-sans" onclick={() => siteSettingsForm.favicon = ''}>
                      削除
                    </button>
                  </div>
                {/if}
              </div>

              <!-- Theme Color Input -->
              <div class="col s12" style="margin-bottom: 24px; display: flex; align-items: center; gap: 16px; flex-wrap: wrap;">
                <div style="flex: 1; min-width: 120px;">
                  <label for="site_theme_color" class="font-sans" style="font-size: 0.8rem; color: var(--wc-text-muted); display: block; margin-bottom: 6px; font-weight: 600;">テーマカラー</label>
                  <input id="site_theme_color" type="color" bind:value={siteSettingsForm.theme_color} style="width: 100%; height: 40px; border: 1.5px solid var(--wc-border); border-radius: 0px; padding: 0; cursor: pointer; background: none;" />
                </div>
                <div style="flex: 3; display: flex; flex-direction: column; gap: 6px; min-width: 200px; text-align: left;">
                  <span class="font-sans" style="font-size: 0.9rem; font-weight: 700; color: var(--wc-text);">現在の色: {siteSettingsForm.theme_color}</span>
                  <button class="reset-color-btn font-sans" onclick={() => siteSettingsForm.theme_color = '#7C6B52'}>
                    デフォルトの煤竹色 (#7C6B52) に戻す
                  </button>
                </div>
              </div>

              <!-- External URL Input -->
              <div class="input-field col s12" style="margin-bottom: 28px;">
                <input id="site_external_url" type="text" class="nm-input" bind:value={siteSettingsForm.external_url} placeholder="https://my.domain/subpass" style="margin-bottom: 0;" />
                <label for="site_external_url" class="active font-sans">外部URL (本番サブパス運用時など設定。空欄時は自動取得)</label>
              </div>

              <!-- Save Button -->
              <div class="col s12 right-align">
                <button class="save-settings-btn font-sans" onclick={handleSaveSiteSettings} disabled={saving}>
                  <i class="material-icons" style="font-size: 1.15rem; margin-right: 6px; vertical-align: middle;">save</i>設定を保存
                </button>
              </div>
            </div>
          </div>
        </div>
      {/if}

      {#each ['google', 'line', 'meta'] as provider}
        {#if activeTab === provider}
          <div class="nm-card admin-panel-card animate-fade-in">
            <div class="card-content" style="padding: 2.5rem;">
              <span class="card-title font-mincho">
                <i class="material-icons title-icon">settings</i> {provider.toUpperCase()} 連携の設定
              </span>

              <div class="row" style="margin-bottom: 0;">
                <!-- Enabled/Disabled Segmented selector -->
                <div class="col s12" style="margin-bottom: 24px; text-align: left;">
                  <label class="font-sans" style="font-size: 0.8rem; color: var(--wc-text-muted); display: block; margin-bottom: 8px; font-weight: 600;">プロバイダ有効化設定</label>
                  <div class="provider-status-selector">
                    <button 
                      type="button" 
                      class="status-tab font-sans" 
                      class:active={!settings[provider].enabled} 
                      onclick={() => settings[provider].enabled = false}
                    >
                      無効
                    </button>
                    <button 
                      type="button" 
                      class="status-tab font-sans" 
                      class:active={settings[provider].enabled} 
                      onclick={() => settings[provider].enabled = true}
                    >
                      有効
                    </button>
                  </div>
                </div>

                <!-- Client ID Input -->
                <div class="input-field col s12" style="margin-bottom: 20px;">
                  <input id="{provider}_client_id" type="text" class="nm-input" bind:value={settings[provider].client_id} placeholder="クライアントID (Client ID)" style="margin-bottom: 0;" />
                  <label for="{provider}_client_id" class="active font-sans">クライアント ID</label>
                </div>

                <!-- Client Secret Input -->
                <div class="input-field col s12" style="margin-bottom: 20px;">
                  <input id="{provider}_client_secret" type="password" class="nm-input" bind:value={settings[provider].client_secret} placeholder="クライアントシークレット (Client Secret)" style="margin-bottom: 0;" />
                  <label for="{provider}_client_secret" class="active font-sans">クライアント シークレット</label>
                </div>

                <!-- Redirect URL (Read-only ticket style) -->
                <div class="col s12" style="margin-bottom: 28px; text-align: left;">
                  <label for="{provider}_redirect_url" class="font-sans" style="font-size: 0.8rem; color: var(--wc-text-muted); display: block; margin-bottom: 6px; font-weight: 600;">リダイレクト URL (読み取り専用)</label>
                  <input id="{provider}_redirect_url" type="text" readonly class="nm-input redirect-url-box font-mono" value={computedRedirectUrl(provider)} style="margin-bottom: 0;" />
                  <span class="helper-text font-sans" style="color: var(--wc-text-muted); font-size: 0.78rem; margin-top: 6px; display: block; line-height: 1.4;">
                    このURLをGoogleやLINE等のデベロッパーコンソールに「認証リダイレクトURI」として登録してください。
                  </span>
                </div>

                <!-- Save Button -->
                <div class="col s12 right-align">
                  <button class="save-settings-btn font-sans" onclick={() => handleSave(provider)} disabled={saving}>
                    <i class="material-icons" style="font-size: 1.15rem; margin-right: 6px; vertical-align: middle;">save</i>設定を保存
                  </button>
                </div>
              </div>
            </div>
          </div>
        {/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  /* Base Vogue Admin Card */
  .admin-panel-card {
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-text) !important;
    background: var(--wc-surface) !important;
    box-shadow: 8px 8px 0px var(--wc-shadow-dark) !important;
  }

  .card-title {
    font-weight: 800;
    font-size: 1.25rem;
    margin-bottom: 2rem;
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--wc-text);
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .title-icon {
    font-size: 1.4rem;
    color: var(--wc-accent);
  }

  /* Vogue sharp Input */
  .nm-input {
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-border) !important;
    background: rgba(245, 240, 232, 0.5) !important;
    color: var(--wc-text) !important;
    padding: 10px 14px !important;
    font-size: 0.95rem !important;
    box-shadow: none !important;
    width: 100%;
    box-sizing: border-box;
  }

  .nm-input:focus {
    border-color: var(--wc-accent) !important;
    outline: none !important;
  }

  .input-field label.active {
    transform: translateY(-12px) scale(0.8);
    left: 0.75rem;
    color: var(--wc-text-muted) !important;
    font-weight: 600;
  }

  /* Sharp Tab Switcher */
  .tabs-container {
    border: 1.5px solid var(--wc-text);
    background: var(--wc-surface);
    box-shadow: 4px 4px 0px var(--wc-shadow-dark);
  }

  .tabs-list {
    display: flex;
  }

  .tab-btn {
    border: none;
    background: transparent;
    padding: 14px 16px;
    font-size: 0.88rem;
    font-weight: 700;
    color: var(--wc-text-muted);
    cursor: pointer;
    transition: var(--wc-transition-fast);
    border-right: 1.5px solid var(--wc-text);
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
  }

  .tab-btn:hover:not(.active) {
    background-color: var(--wc-surface-alt);
    color: var(--wc-text);
  }

  .tab-btn.active {
    background-color: var(--wc-text);
    color: var(--wc-surface) !important;
  }

  .w-100 {
    width: 100%;
  }

  /* Action Buttons - Vogue sharp style */
  .logout-btn {
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-text) !important;
    background: var(--wc-surface) !important;
    color: var(--wc-text) !important;
    box-shadow: 3px 3px 0px var(--wc-text) !important;
    padding: 8px 18px;
    font-weight: 700;
    font-size: 0.88rem;
    cursor: pointer;
    transition: var(--wc-transition-fast);
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .logout-btn:hover {
    transform: translate(-1px, -1px);
    box-shadow: 4px 4px 0px var(--wc-text) !important;
    background: var(--wc-surface-alt) !important;
  }

  .logout-btn:active {
    transform: translate(1px, 1px);
    box-shadow: 1px 1px 0px var(--wc-text) !important;
  }

  .save-settings-btn {
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-text) !important;
    background: var(--wc-accent) !important;
    color: #FFFFFF !important;
    box-shadow: 4px 4px 0px var(--wc-text) !important;
    padding: 10px 24px;
    font-weight: 700;
    font-size: 0.9rem;
    letter-spacing: 0.03em;
    cursor: pointer;
    transition: var(--wc-transition-fast);
    display: inline-flex;
    align-items: center;
  }

  .save-settings-btn:hover:not(:disabled) {
    transform: translate(-1px, -1px);
    box-shadow: 5px 5px 0px var(--wc-text) !important;
    background: var(--wc-accent-hover) !important;
  }

  .save-settings-btn:active:not(:disabled) {
    transform: translate(1px, 1px);
    box-shadow: 2px 2px 0px var(--wc-text) !important;
  }

  .save-settings-btn:disabled {
    opacity: 0.55;
    cursor: not-allowed;
    box-shadow: none !important;
    transform: none !important;
  }

  /* File upload element styling */
  .file-field-container {
    display: flex;
    align-items: stretch;
    border: 1.5px solid var(--wc-border);
    box-shadow: 3px 3px 0px var(--wc-shadow-dark);
  }

  .file-select-btn {
    position: relative;
    overflow: hidden;
    background: var(--wc-surface-alt);
    color: var(--wc-text);
    border-right: 1.5px solid var(--wc-border);
    padding: 10px 18px;
    font-weight: 700;
    font-size: 0.85rem;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .file-select-btn input[type=file] {
    position: absolute;
    top: 0;
    right: 0;
    margin: 0;
    padding: 0;
    font-size: 20px;
    cursor: pointer;
    opacity: 0;
    filter: alpha(opacity=0);
    height: 100%;
    width: 100%;
  }

  .file-path-text {
    flex-grow: 1;
    border: none !important;
    box-shadow: none !important;
    background: transparent !important;
  }

  .delete-btn {
    border: 1.5px solid #b91c1c;
    background: transparent;
    color: #b91c1c;
    padding: 4px 10px;
    font-size: 0.8rem;
    font-weight: 700;
    cursor: pointer;
    transition: var(--wc-transition-fast);
  }

  .delete-btn:hover {
    background: rgba(185, 28, 28, 0.08);
  }

  .reset-color-btn {
    border: none;
    background: transparent;
    padding: 0;
    font-size: 0.82rem;
    color: var(--wc-accent);
    cursor: pointer;
    text-decoration: underline;
    text-underline-offset: 3px;
    align-self: flex-start;
  }

  .reset-color-btn:hover {
    color: var(--wc-accent-hover);
  }

  /* Segmented status selector for provider toggle */
  .provider-status-selector {
    display: inline-flex;
    border: 1.5px solid var(--wc-border);
    background: var(--wc-surface);
  }

  .status-tab {
    border: none;
    background: transparent;
    padding: 8px 20px;
    font-size: 0.85rem;
    font-weight: 700;
    cursor: pointer;
    color: var(--wc-text-muted);
    transition: var(--wc-transition-fast);
  }

  .status-tab:first-child {
    border-right: 1.5px solid var(--wc-border);
  }

  .status-tab.active {
    background: var(--wc-text);
    color: var(--wc-surface) !important;
  }

  .status-tab:hover:not(.active) {
    background: var(--wc-surface-alt);
    color: var(--wc-text);
  }

  .redirect-url-box {
    background-color: var(--wc-surface-alt) !important;
    color: var(--wc-text-muted) !important;
    cursor: not-allowed;
  }

  .font-mincho {
    font-family: 'Shippori Mincho B1', serif;
  }

  .font-sans {
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
  }

  .font-mono {
    font-family: 'JetBrains Mono', monospace;
  }

  .animate-fade-in {
    animation: fadeIn 0.35s ease-out;
  }

  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(12px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>
