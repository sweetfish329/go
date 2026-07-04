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

  let activeTab = $state('google');
  let loading = $state(true);
  let saving = $state(false);

  const getM = () => (window as any).M;

  onMount(async () => {
    await fetchSettings();
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
                  <input id="{provider}_redirect_url" type="text" bind:value={settings[provider].redirect_url} placeholder="https://yourdomain.com/api/auth/callback" />
                  <label for="{provider}_redirect_url" class="active">リダイレクト URL (Callback URL)</label>
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
