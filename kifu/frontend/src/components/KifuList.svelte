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
    try {
      const url = publicMode ? `/api/u/${userId}/kifus` : '/api/kifus';
      const headers = publicMode ? {} : auth.getHeaders();
      const res = await fetch(url, { headers });
      if (!res.ok) throw new Error("Failed to fetch games");
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

  onMount(() => {
    fetchKifus();
  });
</script>

<div class="row">
  <div class="col s12 d-flex justify-between align-center" style="display: flex; justify-content: space-between; align-items: center; margin-top: 1.5rem; margin-bottom: 1rem; flex-wrap: wrap; gap: 10px;">
    <h4 style="margin: 0; font-weight: 700; color: var(--nm-accent);" class="font-pixel">{publicMode ? '公開棋譜ライブラリ' : '棋譜ライブラリ'}</h4>
    {#if !publicMode}
      <div style="display: flex; gap: 12px;">
        <button class="nm-btn-primary y2k-glow-blue y2k-grad-blue font-pixel" onclick={() => dispatch('createKifu')} style="color: #1a1e24 !important;">
          <i class="material-icons" style="font-size: 1.2rem; color: #1a1e24;">edit</i>自分で棋譜を作成
        </button>
        <button class="nm-btn font-pixel" onclick={() => showUploadForm = !showUploadForm}>
          <i class="material-icons" style="font-size: 1.2rem;">{showUploadForm ? 'close' : 'cloud_upload'}</i>
          {showUploadForm ? '閉じる' : 'SGFアップロード'}
        </button>
      </div>
    {/if}
  </div>

  <!-- Filter Panel -->
  {#if !loading && !error && kifus.length > 0}
    <div class="col s12" style="margin-bottom: 1.5rem;">
      <div class="nm-card filter-card animate-fade-in" style="margin: 0;">
        <div class="card-content" style="padding: 20px 24px;">
          <span class="card-title font-pixel" style="font-size: 1.1rem; font-weight: 600; margin-bottom: 16px; display: flex; align-items: center; gap: 8px; color: var(--nm-accent);">
            <i class="material-icons" style="font-size: 1.25rem;">filter_list</i> フィルター検索
          </span>
          <div class="row" style="margin-bottom: 0; display: flex; flex-wrap: wrap; gap: 12px 0; align-items: flex-end;">
            <!-- Text Search -->
            <div class="input-field col s12 m6" style="margin-top: 0; margin-bottom: 0;">
              <i class="material-icons prefix" style="font-size: 1.3rem; margin-top: 10px; color: var(--nm-text-muted);">search</i>
              <input id="search-query" type="text" class="nm-input font-pixel" bind:value={searchQuery} placeholder="対局名、対局者名（黒/白）" style="margin-bottom: 0; padding-left: 3rem !important;" />
              <label for="search-query" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem;">キーワード</label>
            </div>
            <!-- Date Start -->
            <div class="input-field col s6 m3" style="margin-top: 0; margin-bottom: 0;">
              <input id="start-date" type="date" class="nm-input font-pixel" bind:value={startDate} style="margin-bottom: 0;" />
              <label for="start-date" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem;">対局日 (開始)</label>
            </div>
            <!-- Date End -->
            <div class="input-field col s6 m3" style="margin-top: 0; margin-bottom: 0;">
              <input id="end-date" type="date" class="nm-input font-pixel" bind:value={endDate} style="margin-bottom: 0;" />
              <label for="end-date" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem;">対局日 (終了)</label>
            </div>
          </div>
          {#if searchQuery || startDate || endDate}
            <!-- svelte-ignore a11y-missing-attribute -->
            <div class="right-align" style="margin-top: 12px;">
              <a class="cursor-pointer font-pixel" onclick={() => { searchQuery = ""; startDate = ""; endDate = ""; }} style="cursor: pointer; font-size: 0.9rem; font-weight: 500; display: inline-flex; align-items: center; gap: 4px; color: var(--nm-accent);">
                <i class="material-icons" style="font-size: 1.1rem;">clear_all</i>検索条件をクリア
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
          <span class="card-title" style="font-weight: 600; color: var(--nm-accent); margin-bottom: 20px;">SGF棋譜のアップロード</span>
          
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
      <div class="preloader-wrapper big active">
        <div class="spinner-layer spinner-brown-only">
          <div class="circle-clipper left">
            <div class="circle"></div>
          </div><div class="gap-patch">
            <div class="circle"></div>
          </div><div class="circle-clipper right">
            <div class="circle"></div>
          </div>
        </div>
      </div>
      <p class="grey-text">棋譜データを読み込み中...</p>
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
        <div class="nm-card hoverable kifu-card pixel-border-sm waves-effect waves-block animate-pop-in stagger-{(i % 5) + 1}" style="width: 100%; display: block; text-align: left;" onclick={() => dispatch('selectKifu', k.id)}>
          <div class="card-content" style="padding: 20px 24px; position: relative;">
            {#if k.result}
              <div class="y2k-sticker pink font-pixel" style="position: absolute; top: 12px; right: -5px; z-index: 5; font-size: 0.7rem; padding: 2px 8px;">
                {k.result}
              </div>
            {/if}
            <span class="card-title truncate font-pixel" style="font-weight: 700; font-size: 1.15rem; margin-bottom: 0.8rem; color: var(--nm-accent); max-width: calc(100% - 80px);" title={k.title}>
              {k.title}
            </span>
            
            <div class="players-info" style="margin: 0.8rem 0; display: flex; flex-direction: column; gap: 8px;">
              <div class="player black-player d-flex align-center" style="display: flex; align-items: center;">
                <span class="stone-badge black-stone" style="display: inline-block; width: 14px; height: 14px; border-radius: 50%; background: radial-gradient(circle at 30% 30%, #555, #111); margin-right: 10px; border: 1px solid #000; box-shadow: 2px 2px 4px rgba(0,0,0,0.4);"></span>
                <span class="font-pixel" style="font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 150px; color: var(--nm-text-main);">{k.black_player || 'Unknown'}</span>
                {#if k.black_rank}
                  <span class="nm-badge-inset font-pixel" style="font-size: 0.7rem; margin-left: 8px; padding: 1px 6px;">{k.black_rank}</span>
                {/if}
              </div>
              <div class="player white-player d-flex align-center" style="display: flex; align-items: center;">
                <span class="stone-badge white-stone" style="display: inline-block; width: 14px; height: 14px; border-radius: 50%; background: radial-gradient(circle at 30% 30%, #fff, #ddd); margin-right: 10px; border: 1px solid #bbb; box-shadow: 1px 1px 3px rgba(0,0,0,0.15), -1px -1px 2px rgba(255,255,255,0.8);"></span>
                <span class="font-pixel" style="font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 150px; color: var(--nm-text-main);">{k.white_player || 'Unknown'}</span>
                {#if k.white_rank}
                  <span class="nm-badge-inset font-pixel" style="font-size: 0.7rem; margin-left: 8px; padding: 1px 6px;">{k.white_rank}</span>
                {/if}
              </div>
            </div>

            <div class="game-meta font-pixel" style="font-size: 0.8rem; line-height: 1.5; color: var(--nm-text-muted); margin-top: 12px; border-top: 1px dashed rgba(163, 177, 198, 0.3); padding-top: 8px;">
              <div>対局日: {k.game_date || '不明'}</div>
              <div style="font-size: 0.7rem; margin-top: 4px;">登録日: {new Date(k.created_at).toLocaleDateString('ja-JP')}</div>
            </div>
          </div>
          
          <div class="card-action d-flex justify-between" style="display: flex; justify-content: space-between; align-items: center; background-color: transparent; border-top: 1px solid rgba(163, 177, 198, 0.2); padding: 12px 20px;">
            <span class="font-pixel" style="font-weight: 700; color: var(--nm-accent); font-size: 0.85rem; display: inline-flex; align-items: center; gap: 4px;">
              開く <i class="material-icons" style="font-size: 1.1rem;">arrow_forward</i>
            </span>
            {#if !publicMode}
              <button class="nm-btn-flat nm-btn-round" style="width: 32px; height: 32px; min-width: 32px; padding: 0;" onclick={(e) => handleDelete(k.id, e)} title="削除">
                <i class="material-icons red-text text-lighten-1" style="font-size: 1.2rem;">delete</i>
              </button>
            {/if}
          </div>
        </div>
      </div>
    {/each}
  {/if}
</div>

<style>
  .kifu-card {
    cursor: pointer;
    transition: transform 0.2s ease, box-shadow 0.2s ease;
    border-radius: 8px;
    overflow: hidden;
  }
  .kifu-card:hover {
    transform: translateY(-2px);
  }
  .spinner-brown-only {
    border-color: #5d4037 !important;
  }
  .animate-fade-in {
    animation: fadeIn 0.3s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(10px); }
    to { opacity: 1; transform: translateY(0); }
  }

  /* Mobile responsive adjustments */
  @media only screen and (max-width: 600px) {
    :global(.card-content) {
      padding: 14px !important;
    }
    .players-info {
      margin: 0.5rem 0 !important;
    }
    h4 {
      font-size: 1.8rem !important;
    }
  }
</style>
