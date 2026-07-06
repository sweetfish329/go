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
  <div class="col s12 d-flex justify-between align-center" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem;">
    <h4 style="margin: 0; font-weight: 500;" class="brown-text text-darken-3">OAuthプロバイダ設定 (管理)</h4>
    <button class="btn-flat waves-effect waves-red" onclick={handleLogout}>
      <i class="material-icons left">exit_to_app</i>管理ログアウト
    </button>
  </div>

  {#if loading}
    <div class="col s12 center-align" style="margin-top: 4rem;">
      <div class="preloader-wrapper big active">
        <div class="spinner-layer spinner-brown-only">
          <div class="circle-clipper left"><div class="circle"></div></div>
          <div class="gap-patch"><div class="circle"></div></div>
          <div class="circle-clipper right"><div class="circle"></div></div>
        </div>
      </div>
    </div>
  {:else}
    <!-- Tabs Nav -->
    <div class="col s12" style="margin-bottom: 1.5rem;">
      <div class="card-panel white" style="padding: 0; border-radius: 8px; box-shadow: 0 4px 15px rgba(0,0,0,0.04);">
        <ul class="tabs-list" style="display: flex; list-style: none; margin: 0; padding: 0; border-bottom: 1px solid #e0e0e0;">
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
        <div class="card white animate-fade-in" style="border-radius: 12px; box-shadow: 0 8px 24px rgba(0,0,0,0.05); border: 1px solid rgba(0,0,0,0.03);">
          <div class="card-content" style="padding: 2rem;">
            <span class="card-title brown-text text-darken-3" style="font-weight: 500; font-size: 1.3rem; margin-bottom: 1.5rem; display: flex; align-items: center; gap: 8px;">
              <i class="material-icons">web</i> サイト基本設定
            </span>

            <div class="row">
              <div class="input-field col s12" style="margin-bottom: 1.5rem;">
                <input id="site_title" type="text" bind:value={siteSettingsForm.title} placeholder="kifu_store" />
                <label for="site_title" class="active">ページ右上のタイトル</label>
              </div>

              <div class="input-field col s12" style="margin-bottom: 1.5rem;">
                <input id="site_tab_name" type="text" bind:value={siteSettingsForm.tab_name} placeholder="kifu_store" />
                <label for="site_tab_name" class="active">タブ名 (ブラウザのタイトル)</label>
              </div>

              <div class="col s12" style="margin-bottom: 1.5rem;">
                <label style="font-size: 0.8rem; color: #9e9e9e;">ファビコン (Favicon)</label>
                <div class="file-field input-field" style="margin-top: 0;">
                  <div class="btn brown darken-1">
                    <span>画像を選択</span>
                    <input type="file" accept="image/*" onchange={handleFaviconChange} />
                  </div>
                  <div class="file-path-wrapper">
                    <input class="file-path validate" type="text" placeholder="ファビコン用の画像ファイルをアップロードしてください (.ico, .png, .svg など)" />
                  </div>
                </div>
                {#if siteSettingsForm.favicon}
                  <div style="margin-top: 8px; display: flex; align-items: center; gap: 12px;">
                    <span style="font-size: 0.9rem; color: #555;">プレビュー:</span>
                    <img src={siteSettingsForm.favicon} alt="Favicon Preview" style="width: 32px; height: 32px; object-fit: contain; border: 1px solid #ddd; padding: 2px; border-radius: 4px; background: #fff;" />
                    <button class="btn-flat waves-effect red-text text-darken-2" onclick={() => siteSettingsForm.favicon = ''} style="padding: 0 8px;">
                      削除
                    </button>
                  </div>
                {/if}
              </div>

              <div class="col s12" style="margin-bottom: 2rem; display: flex; align-items: center; gap: 16px;">
                <div style="flex: 1;">
                  <label for="site_theme_color" style="font-size: 0.8rem; color: #9e9e9e; display: block; margin-bottom: 4px;">テーマカラー</label>
                  <input id="site_theme_color" type="color" bind:value={siteSettingsForm.theme_color} style="width: 100%; height: 40px; border: 1px solid #ccc; border-radius: 4px; padding: 0; cursor: pointer; background: none;" />
                </div>
                <div style="flex: 3; display: flex; flex-direction: column; gap: 4px;">
                  <span style="font-size: 0.95rem; font-weight: 500; color: #555;">現在の色: {siteSettingsForm.theme_color}</span>
                  <button class="btn-flat waves-effect brown-text" onclick={() => siteSettingsForm.theme_color = '#4e342e'} style="align-self: flex-start; padding: 0; font-size: 0.85rem; height: auto; line-height: normal;">
                    デフォルトの茶色 (#4e342e) に戻す
                  </button>
                </div>
              </div>

              <div class="input-field col s12" style="margin-bottom: 2rem;">
                <input id="site_external_url" type="text" bind:value={siteSettingsForm.external_url} placeholder="https://my.domain/subpass" />
                <label for="site_external_url" class="active">外部URL (本番サブパス運用時などに設定。空欄時は自動取得)</label>
              </div>

              <div class="col s12 right-align">
                <button class="btn waves-effect waves-light brown darken-2" onclick={handleSaveSiteSettings} disabled={saving}>
                  <i class="material-icons left">save</i>設定を保存
                </button>
              </div>
            </div>
          </div>
        </div>
      {/if}

      {#each ['google', 'line', 'meta'] as provider}
        {#if activeTab === provider}
          <div class="card white animate-fade-in" style="border-radius: 12px; box-shadow: 0 8px 24px rgba(0,0,0,0.05); border: 1px solid rgba(0,0,0,0.03);">
            <div class="card-content" style="padding: 2rem;">
              <span class="card-title brown-text text-darken-3" style="font-weight: 500; font-size: 1.3rem; margin-bottom: 1.5rem; display: flex; align-items: center; gap: 8px;">
                <i class="material-icons">settings</i> {provider.toUpperCase()} 連携の設定
              </span>

              <div class="row">
                <div class="col s12" style="margin-bottom: 1.5rem;">
                  <div class="switch">
                    <label style="font-size: 1rem; font-weight: 500; color: #555;">
                      このプロバイダを無効
                      <input type="checkbox" bind:checked={settings[provider].enabled} />
                      <span class="lever"></span>
                      有効にする
                    </label>
                  </div>
                </div>

                <div class="input-field col s12" style="margin-bottom: 1.5rem;">
                  <input id="{provider}_client_id" type="text" bind:value={settings[provider].client_id} placeholder="クライアントID (Client ID)" />
                  <label for="{provider}_client_id" class="active">クライアント ID</label>
                </div>

                <div class="input-field col s12" style="margin-bottom: 1.5rem;">
                  <input id="{provider}_client_secret" type="password" bind:value={settings[provider].client_secret} placeholder="クライアントシークレット (Client Secret)" />
                  <label for="{provider}_client_secret" class="active">クライアント シークレット</label>
                </div>

                <div class="input-field col s12" style="margin-bottom: 2rem;">
                  <input id="{provider}_redirect_url" type="text" readonly value={computedRedirectUrl(provider)} style="background-color: #f5f5f5; color: #777;" />
                  <label for="{provider}_redirect_url" class="active">リダイレクト URL (コピペ用・読み取り専用)</label>
                  <span class="helper-text" style="color: #888;">
                    このURLをGoogleやLINE等のデベロッパーコンソールに「リダイレクトURI」として登録してください。
                  </span>
                </div>

                <div class="col s12 right-align">
                  <button class="btn waves-effect waves-light brown darken-2" onclick={() => handleSave(provider)} disabled={saving}>
                    <i class="material-icons left">save</i>設定を保存
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
  .animate-fade-in {
    animation: fadeIn 0.3s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(10px); }
    to { opacity: 1; transform: translateY(0); }
  }

  .tabs-list {
    border-radius: 8px 8px 0 0;
    overflow: hidden;
  }

  .tab-btn {
    border: none;
    background: transparent;
    padding: 1rem;
    font-size: 1.05rem;
    font-weight: 500;
    color: #777;
    cursor: pointer;
    transition: all 0.2s;
    border-bottom: 3px solid transparent;
  }
  .tab-btn:hover {
    background-color: #f9f9f9;
    color: #5d4037;
  }
  .tab-btn.active {
    color: #5d4037;
    border-bottom-color: #5d4037;
    background-color: #fafafa;
  }
  .w-100 {
    width: 100%;
  }
</style>
