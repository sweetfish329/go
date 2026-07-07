<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte';
  import { auth } from '../lib/auth.svelte';

  interface KifuItem {
    id: string;
    title: string;
    black_player?: string;
    black_rank?: string;
    white_player?: string;
    white_rank?: string;
    result?: string;
    game_date?: string;
    created_at: string;
  }

  const dispatch = createEventDispatcher<{
    selectKifu: string;
    createKifu: void;
  }>();

  let { userId = "" } = $props<{
    userId?: string;
  }>();

  let publicMode = $derived(!!userId);

  // Reactive states using Svelte 5 Runes
  let kifus = $state<KifuItem[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let ownerUsername = $state<string | null>(null);

  // Form states
  let title = $state("");
  let sgfData = $state("");
  let showUploadForm = $state(false);
  let uploading = $state(false);

  // Filter states
  let searchQuery = $state("");
  let startDate = $state("");
  let endDate = $state("");

  // Derived filtered kifu list
  let filteredKifus = $derived.by(() => {
    return kifus.filter(k => {
      // 1. Text search
      if (searchQuery.trim()) {
        const query = searchQuery.toLowerCase();
        const titleMatch = k.title?.toLowerCase().includes(query);
        const blackMatch = k.black_player?.toLowerCase().includes(query);
        const whiteMatch = k.white_player?.toLowerCase().includes(query);
        if (!titleMatch && !blackMatch && !whiteMatch) {
          return false;
        }
      }

      // 2. Date range filtering (game_date is YYYY-MM-DD)
      const gameDateStr = k.game_date;
      if (gameDateStr) {
        if (startDate && gameDateStr < startDate) return false;
        if (endDate && gameDateStr > endDate) return false;
      } else {
        // Exclude if filter is active but game_date is missing
        if (startDate || endDate) return false;
      }

      return true;
    });
  });

  // Type helper for Materialize global object
  const getM = () => (window as any).M;

  async function fetchKifus(): Promise<void> {
    loading = true;
    error = null; // 前回のエラーをクリア
    try {
      const url = publicMode ? `/api/u/${userId}/kifus` : '/api/kifus';
      const headers = publicMode ? {} : auth.getHeaders();
      const res = await fetch(url, { headers });
      if (!res.ok) {
        if (res.status === 401) {
          // トークンが無効・期限切れの場合は再ログイン促す
          throw new Error("認証エラー: 再ログインしてください");
        }
        throw new Error(`棋譜の取得に失敗しました (${res.status})`);
      }
      kifus = await res.json();
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function handleUpload(): Promise<void> {
    if (!sgfData.trim()) return;

    uploading = true;
    try {
      const res = await fetch('/api/kifus', {
        method: 'POST',
        headers: auth.getHeaders(),
        body: JSON.stringify({
          title: title.trim() || undefined,
          sgf_data: sgfData.trim()
        })
      });

      if (!res.ok) {
        const errJson = await res.json();
        throw new Error(errJson.error || "Failed to upload game");
      }

      // Reset form
      title = "";
      sgfData = "";
      showUploadForm = false;
      
      // Reload games
      await fetchKifus();
      
      const M = getM();
      if (M) {
        M.toast({ html: '棋譜が登録されました！', classes: 'green darken-1' });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + err.message, classes: 'red darken-1' });
      }
    } finally {
      uploading = false;
    }
  }

  async function handleDelete(id: string, e: MouseEvent): Promise<void> {
    e.stopPropagation(); // Avoid triggering card click (selecting kifu)
    if (!confirm("本当にこの棋譜を削除しますか？")) return;

    try {
      const res = await fetch(`/api/kifus/${id}`, {
        method: 'DELETE',
        headers: auth.getHeaders()
      });
      if (!res.ok) throw new Error("Failed to delete game");
      
      kifus = kifus.filter(k => k.id !== id);
      const M = getM();
      if (M) {
        M.toast({ html: '棋譜が削除されました', classes: 'grey darken-3' });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + err.message, classes: 'red' });
      }
    }
  }

  // Handle local SGF file upload
  function handleFileChange(event: Event): void {
    const target = event.target as HTMLInputElement;
    const file = target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (e: ProgressEvent<FileReader>) => {
      const result = e.target?.result;
      if (typeof result === 'string') {
        sgfData = result;
        if (!title) {
          // Use filename without extension as default title
          title = file.name.replace(/\.sgf$/i, '');
        }
      }
    };
    reader.readAsText(file);
  }

  async function fetchOwnerUsername(): Promise<void> {
    if (!publicMode || !userId) return;
    try {
      const res = await fetch(`/api/users/${userId}/username`);
      if (res.ok) {
        const data = await res.json();
        ownerUsername = data.username;
      }
    } catch (err) {
      console.error("Failed to fetch owner username:", err);
    }
  }

  onMount(() => {
    fetchKifus();
    fetchOwnerUsername();
  });
</script>

<div class="row">
  <!-- Page Header -->
  <div class="col s12" style="margin-top: 1.5rem; margin-bottom: 1.5rem;">
    <div class="list-page-header">
      <div class="list-page-title-wrap">
        <h1 class="list-page-title font-outfit">
          {#if publicMode}
            {ownerUsername ? `✦ ${ownerUsername}'s Kifu` : '✦ Public Kifu'}
          {:else}
            {auth.username ? `✦ ${auth.username}'s Kifu` : '✦ My Kifu'}
          {/if}
        </h1>
        <p class="list-page-subtitle text-muted">
          {#if publicMode}
            公開棋譜ライブラリ
          {:else}
            あなたの棋譜コレクション
          {/if}
        </p>
      </div>
      {#if !publicMode}
        <div class="list-page-actions">
          <button class="nm-btn-primary" onclick={() => dispatch('createKifu')}>
            <i class="material-icons" style="font-size: 1.1rem;">edit</i>棋譜を作成
          </button>
          <button class="nm-btn" onclick={() => showUploadForm = !showUploadForm}>
            <i class="material-icons" style="font-size: 1.1rem;">{showUploadForm ? 'close' : 'cloud_upload'}</i>
            {showUploadForm ? '閉じる' : 'SGF Upload'}
          </button>
        </div>
      {/if}
    </div>
  </div>

  <!-- Filter Panel -->
  {#if !loading && !error && kifus.length > 0}
    <div class="col s12" style="margin-bottom: 1.5rem;">
      <div class="nm-card filter-card animate-fade-in" style="margin: 0;">
        <div class="card-content" style="padding: 18px 24px;">
          <div class="filter-header">
            <span class="filter-label font-outfit">
              <i class="material-icons" style="font-size: 1rem; vertical-align: middle;">filter_list</i>
              フィルター
            </span>
          </div>
          <div class="row" style="margin-bottom: 0; display: flex; flex-wrap: wrap; gap: 12px 16px; align-items: flex-end;">
            <!-- Text Search -->
            <div class="input-field col s12 m6" style="margin: 0;">
              <i class="material-icons prefix" style="font-size: 1.2rem; margin-top: 10px; color: var(--wc-accent); opacity: 0.7;">search</i>
              <input id="search-query" type="text" class="nm-input" bind:value={searchQuery} placeholder="タイトル・対局者名" style="margin-bottom: 0; padding-left: 3rem !important;" />
              <label for="search-query" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--nm-text-muted);">キーワード検索</label>
            </div>
            <!-- Date Start -->
            <div class="input-field col s6 m3" style="margin: 0;">
              <input id="start-date" type="date" class="nm-input" bind:value={startDate} style="margin-bottom: 0;" />
              <label for="start-date" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--nm-text-muted);">開始日</label>
            </div>
            <!-- Date End -->
            <div class="input-field col s6 m3" style="margin: 0;">
              <input id="end-date" type="date" class="nm-input" bind:value={endDate} style="margin-bottom: 0;" />
              <label for="end-date" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--nm-text-muted);">終了日</label>
            </div>
          </div>
          {#if searchQuery || startDate || endDate}
            <div class="right-align" style="margin-top: 10px;">
              <!-- svelte-ignore a11y-missing-attribute -->
              <a class="clear-filter-btn" onclick={() => { searchQuery = ""; startDate = ""; endDate = ""; }}>
                <i class="material-icons" style="font-size: 1rem;">clear_all</i>クリア
              </a>
            </div>
          {/if}
        </div>
      </div>
    </div>
  {/if}

  <!-- Upload Form -->
  {#if showUploadForm}
    <div class="col s12" style="margin-bottom: 1.5rem;">
      <div class="nm-card animate-fade-in">
        <div class="card-content" style="padding: 24px;">
          <span class="card-title" style="font-weight: 600; color: var(--wc-accent); margin-bottom: 20px; font-family: 'Shippori Mincho B1', serif;">SGF棋譜のアップロード</span>
          
          <div class="row" style="margin-bottom: 0; display: flex; flex-wrap: wrap; gap: 12px 0;">
            <div class="file-field input-field col s12 m6" style="margin-top: 0; margin-bottom: 0; display: flex; gap: 10px; align-items: center;">
              <div class="nm-btn" style="position: relative; overflow: hidden; white-space: nowrap;">
                <span>SGFファイル選択</span>
                <input type="file" accept=".sgf" onchange={handleFileChange} />
              </div>
              <div class="file-path-wrapper" style="flex-grow: 1; padding-left: 0;">
                <input class="file-path validate nm-input" type="text" placeholder="または以下に直接貼り付け" style="margin-bottom: 0;" />
              </div>
            </div>

            <div class="input-field col s12 m6" style="margin-top: 0; margin-bottom: 0;">
              <input id="kifu_title" type="text" class="nm-input" bind:value={title} placeholder="対局名など (省略時は自動設定)" style="margin-bottom: 0;" />
              <label for="kifu_title" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem;">タイトル</label>
            </div>

            <div class="input-field col s12" style="margin-top: 12px; margin-bottom: 0;">
              <textarea id="sgf_data" class="materialize-textarea nm-textarea nm-input" style="font-family: monospace; min-height: 120px; margin-bottom: 0;" bind:value={sgfData} placeholder="(;GM[1]FF[4]...)"></textarea>
              <label for="sgf_data" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem;">SGFデータ (必須)</label>
            </div>
          </div>
        </div>
        <div class="card-action right-align" style="border-top: 1px solid rgba(163, 177, 198, 0.2); padding: 16px 24px; display: flex; justify-content: flex-end; gap: 12px;">
          <button class="nm-btn-flat" onclick={() => { showUploadForm = false; title = ""; sgfData = ""; }}>キャンセル</button>
          <button class="nm-btn-primary" disabled={!sgfData.trim() || uploading} onclick={handleUpload}>
            {#if uploading}
              保存中...
            {:else}
              <i class="material-icons" style="font-size: 1.2rem;">check</i>登録する
            {/if}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Loading State -->
  {#if loading}
    <div class="col s12 center-align" style="margin-top: 5rem;">
      <div class="nm-spinner" style="width: 48px; height: 48px; margin: 0 auto;"></div>
      <p class="text-muted" style="margin-top: 16px; font-size: 0.9rem; font-family: 'DM Sans', sans-serif;">棋譜データを読み込み中...</p>
    </div>
  {:else if error}
    <div class="col s12">
      <div class="card-panel red lighten-4 red-text text-darken-4">
        <i class="material-icons left">error</i>
        エラーが発生しました: {error}
      </div>
    </div>
  {:else if kifus.length === 0}
    <div class="col s12 center-align" style="margin-top: 4rem; padding: 2rem;">
      <i class="material-icons grey-text" style="font-size: 5rem;">layers_clear</i>
      <h5 class="grey-text text-darken-1">登録されている棋譜がありません</h5>
      {#if !publicMode}
        <p class="grey-text">「SGFアップロード」または「自分で棋譜を作成」ボタンから登録を行ってください。</p>
      {:else}
        <p class="grey-text">このユーザーが一般公開している棋譜はまだありません。</p>
      {/if}
    </div>
  {:else if filteredKifus.length === 0}
    <div class="col s12 center-align" style="margin-top: 4rem; padding: 2rem;">
      <i class="material-icons grey-text" style="font-size: 5rem;">search_off</i>
      <h5 class="grey-text text-darken-1">条件に一致する棋譜が見つかりません</h5>
      <p class="grey-text">検索キーワードや日付の範囲を変更してお試しください。</p>
    </div>
  {:else}
      <!-- Kifu Cards Grid -->
    {#each filteredKifus as k, i (k.id)}
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <div class="col s12 m6 l4" style="margin-bottom: 1.5rem;">
        <div
          class="nm-card hoverable kifu-card kifu-stone-hover animate-pop-in stagger-{(i % 5) + 1}"
          style="width: 100%; display: block; text-align: left;"
          onclick={() => dispatch('selectKifu', k.id)}
        >
          <div class="card-content" style="padding: 20px 22px; position: relative;">
            <!-- Result Sticker -->
            {#if k.result}
              <div class="wc-result-badge" style="position: absolute; top: 14px; right: 14px; z-index: 5;">
                {k.result}
              </div>
            {/if}

            <!-- Title -->
            <div class="kifu-card-title" title={k.title}>
              {k.title}
            </div>

            <!-- Players -->
            <div class="players-info">
              <!-- Black Player -->
              <div class="player-row">
                <span class="stone-dot stone-black" aria-label="黒"></span>
                <span class="player-name">{k.black_player || 'Unknown'}</span>
                {#if k.black_rank}
                  <span class="holo-tag">{k.black_rank}</span>
                {/if}
              </div>
              <!-- White Player -->
              <div class="player-row">
                <span class="stone-dot stone-white" aria-label="白"></span>
                <span class="player-name">{k.white_player || 'Unknown'}</span>
                {#if k.white_rank}
                  <span class="holo-tag">{k.white_rank}</span>
                {/if}
              </div>
            </div>

            <!-- Meta Info -->
            <div class="kifu-meta">
              <span>📅 {k.game_date || '対局日不明'}</span>
              <span style="opacity: 0.5; font-size: 0.7rem;">登録: {new Date(k.created_at).toLocaleDateString('ja-JP')}</span>
            </div>
          </div>

          <div class="card-action-bar">
            <span class="open-label">
              開いて再生 <span style="font-size: 1rem;">→</span>
            </span>
            {#if !publicMode}
              <button
                class="delete-btn"
                onclick={(e) => handleDelete(k.id, e)}
                title="削除"
                aria-label="この棋譜を削除"
              >
                <i class="material-icons" style="font-size: 1.1rem;">delete_outline</i>
              </button>
            {/if}
          </div>
        </div>
      </div>
    {/each}
  {/if}
</div>

<style>
  /* ---- Page Header ---- */
  .list-page-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    flex-wrap: wrap;
    gap: 12px;
  }

  .list-page-title-wrap {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .list-page-title {
    margin: 0 !important;
    font-size: 2rem !important;
    font-weight: 600 !important;
    letter-spacing: 0.02em;
    color: var(--wc-text);
    font-family: 'Shippori Mincho B1', 'Noto Serif JP', serif;
    line-height: 1.3 !important;
  }

  .list-page-subtitle {
    margin: 0;
    font-size: 0.85rem;
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
    color: var(--wc-text-muted);
  }

  .list-page-actions {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
    align-items: center;
  }

  /* ---- Filter ---- */
  .filter-card {
    border-left: 3px solid var(--wc-accent-warm) !important;
  }

  .filter-header {
    display: flex;
    align-items: center;
    margin-bottom: 14px;
  }

  .filter-label {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--wc-accent);
    display: inline-flex;
    align-items: center;
    gap: 5px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    font-family: 'DM Sans', sans-serif;
  }

  .clear-filter-btn {
    cursor: pointer;
    font-size: 0.82rem;
    font-weight: 600;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--wc-accent);
    font-family: 'DM Sans', sans-serif;
  }

  .clear-filter-btn:hover {
    text-decoration: underline;
    text-underline-offset: 3px;
  }

  /* ---- Kifu Cards ---- */
  .kifu-card {
    cursor: pointer;
    transition: transform 0.28s cubic-bezier(0.34, 1.56, 0.64, 1), box-shadow 0.28s ease;
    border-radius: 20px !important;
    overflow: hidden;
  }

  .kifu-card-title {
    font-weight: 600;
    font-size: 1.05rem;
    color: var(--wc-text);
    margin-bottom: 0.75rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: calc(100% - 80px);
    letter-spacing: 0.01em;
    font-family: 'Shippori Mincho B1', 'Noto Serif JP', serif;
  }

  /* ---- Players ---- */
  .players-info {
    margin: 0.75rem 0;
    display: flex;
    flex-direction: column;
    gap: 7px;
  }

  .player-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  /* Stone dots — washi clay style */
  .stone-dot {
    display: inline-block;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .stone-black {
    background: radial-gradient(circle at 32% 32%, #666, #0a0a0a);
    border: 1.5px solid rgba(0,0,0,0.7);
    box-shadow: 1px 1px 4px rgba(0,0,0,0.5), inset -1px -1px 2px rgba(255,255,255,0.1);
  }

  .stone-white {
    background: radial-gradient(circle at 32% 32%, #ffffff, #d4d4d4);
    border: 1.5px solid rgba(180,180,180,0.8);
    box-shadow: 1px 1px 3px rgba(0,0,0,0.15), inset -1px -1px 2px rgba(255,255,255,0.9);
  }

  .player-name {
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
    font-weight: 500;
    font-size: 0.9rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 140px;
    color: var(--wc-text);
  }

  /* ---- Meta ---- */
  .kifu-meta {
    display: flex;
    flex-direction: column;
    gap: 3px;
    font-size: 0.78rem;
    color: var(--wc-text-muted);
    margin-top: 10px;
    padding-top: 8px;
    border-top: 1px solid var(--wc-border);
    font-family: 'DM Sans', sans-serif;
  }

  /* ---- Card Action Bar ---- */
  .card-action-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-top: 1px solid var(--wc-border);
    padding: 10px 20px;
  }

  .open-label {
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
    font-weight: 600;
    font-size: 0.82rem;
    color: var(--wc-accent);
    display: inline-flex;
    align-items: center;
    gap: 5px;
    letter-spacing: 0.04em;
  }

  .delete-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 30px;
    height: 30px;
    border-radius: 50%;
    border: none;
    background: transparent;
    color: var(--wc-text-muted);
    cursor: pointer;
    transition: all 0.2s ease;
    padding: 0;
  }

  .delete-btn:hover {
    background: rgba(180, 60, 60, 0.1);
    color: #B03030;
    transform: scale(1.1);
  }

  /* ---- Result Badge ---- */
  .wc-result-badge {
    background: var(--wc-surface);
    border: 1px solid var(--wc-border);
    box-shadow: var(--nm-shadow-outset-sm);
    padding: 3px 10px;
    border-radius: var(--wc-radius-sm);
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--wc-accent);
    font-family: 'DM Sans', 'JetBrains Mono', sans-serif;
    letter-spacing: 0.04em;
  }

  /* Mobile responsive */
  @media only screen and (max-width: 600px) {
    :global(.card-content) {
      padding: 14px !important;
    }
    .list-page-title {
      font-size: 1.5rem !important;
    }
    .players-info {
      margin: 0.5rem 0 !important;
    }
  }

  .animate-fade-in {
    animation: fadeIn 0.3s ease-out;
  }

  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(8px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>
