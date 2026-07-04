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
  }>();
  let kifus: KifuItem[] = [];
  let loading: boolean = true;
  let error: string | null = null;

  // Form states
  let title: string = "";
  let sgfData: string = "";
  let showUploadForm: boolean = false;
  let uploading: boolean = false;

  // Type helper for Materialize global object
  const getM = () => (window as any).M;

  async function fetchKifus(): Promise<void> {
    loading = true;
    try {
      const res = await fetch('/api/kifus', {
        headers: auth.getHeaders()
      });
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
  <div class="col s12 d-flex justify-between align-center" style="display: flex; justify-content: space-between; align-items: center; margin-top: 1.5rem; margin-bottom: 1rem;">
    <h4 style="margin: 0; font-weight: 500;" class="brown-text text-darken-3">棋譜ライブラリ</h4>
    <button class="btn waves-effect waves-light brown darken-2" on:click={() => showUploadForm = !showUploadForm}>
      <i class="material-icons left">{showUploadForm ? 'close' : 'cloud_upload'}</i>
      {showUploadForm ? '閉じる' : '新規登録'}
    </button>
  </div>

  <!-- Upload Form -->
  {#if showUploadForm}
    <div class="col s12">
      <div class="card white animate-fade-in">
        <div class="card-content">
          <span class="card-title brown-text text-darken-3" style="font-weight: 500;">SGF棋譜のアップロード</span>
          
          <div class="row" style="margin-bottom: 0;">
            <div class="file-field input-field col s12 m6">
              <div class="btn brown lighten-1">
                <span>SGFファイル選択</span>
                <input type="file" accept=".sgf" on:change={handleFileChange} />
              </div>
              <div class="file-path-wrapper">
                <input class="file-path validate" type="text" placeholder="または以下のテキストエリアに直接貼り付けてください" />
              </div>
            </div>

            <div class="input-field col s12 m6">
              <input id="kifu_title" type="text" bind:value={title} placeholder="対局名など (省略時は対局者名から自動設定)" />
              <label for="kifu_title" class="active">タイトル</label>
            </div>

            <div class="input-field col s12">
              <textarea id="sgf_data" class="materialize-textarea" style="font-family: monospace; min-height: 100px;" bind:value={sgfData} placeholder="(;GM[1]FF[4]...)"></textarea>
              <label for="sgf_data" class="active">SGFデータ (必須)</label>
            </div>
          </div>
        </div>
        <div class="card-action right-align" style="background-color: #fafafa;">
          <button class="btn-flat waves-effect" on:click={() => { showUploadForm = false; title = ""; sgfData = ""; }}>キャンセル</button>
          <button class="btn waves-effect waves-light brown" disabled={!sgfData.trim() || uploading} on:click={handleUpload}>
            {#if uploading}
              保存中...
            {:else}
              <i class="material-icons left">check</i>登録する
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
      <p class="grey-text">「新規登録」ボタンからSGFファイルをアップロードして開始してください。</p>
    </div>
  {:else}
    <!-- Kifu Cards Grid -->
    {#each kifus as k (k.id)}
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <div class="col s12 m6 l4">
        <div class="card hoverable kifu-card waves-effect waves-block" style="width: 100%; display: block; text-align: left;" on:click={() => dispatch('selectKifu', k.id)}>
          <div class="card-content">
            <span class="card-title truncate brown-text text-darken-4" style="font-weight: 500; font-size: 1.25rem; margin-bottom: 0.5rem;" title={k.title}>
              {k.title}
            </span>
            
            <div class="players-info" style="margin: 0.8rem 0;">
              <div class="player black-player d-flex align-center" style="display: flex; align-items: center; margin-bottom: 0.25rem;">
                <span class="stone-badge black-stone" style="display: inline-block; width: 12px; height: 12px; border-radius: 50%; background-color: #333; margin-right: 8px; border: 1px solid #000;"></span>
                <span style="font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 180px;">{k.black_player || 'Unknown'}</span>
                {#if k.black_rank}
                  <span class="grey-text text-darken-1" style="font-size: 0.85rem; margin-left: 6px;">({k.black_rank})</span>
                {/if}
              </div>
              <div class="player white-player d-flex align-center" style="display: flex; align-items: center;">
                <span class="stone-badge white-stone" style="display: inline-block; width: 12px; height: 12px; border-radius: 50%; background-color: #fff; margin-right: 8px; border: 1px solid #ccc;"></span>
                <span style="font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 180px;">{k.white_player || 'Unknown'}</span>
                {#if k.white_rank}
                  <span class="grey-text text-darken-1" style="font-size: 0.85rem; margin-left: 6px;">({k.white_rank})</span>
                {/if}
              </div>
            </div>

            <div class="game-meta grey-text text-darken-1" style="font-size: 0.9rem; line-height: 1.4;">
              <div>結果: <span class="brown-text text-darken-1" style="font-weight: 500;">{k.result || 'なし'}</span></div>
              <div>対局日: {k.game_date || '不明'}</div>
              <div style="font-size: 0.8rem; margin-top: 0.4rem;" class="grey-text">アップロード: {new Date(k.created_at).toLocaleDateString('ja-JP')}</div>
            </div>
          </div>
          
          <div class="card-action d-flex justify-between" style="display: flex; justify-content: space-between; align-items: center; background-color: #fafafa; padding: 8px 20px;">
            <span class="brown-text text-darken-2" style="font-weight: 500;">開く <i class="material-icons right" style="vertical-align: middle; font-size: 1.1rem; line-height: inherit;">arrow_forward</i></span>
            <button class="btn-flat btn-floating waves-effect waves-red" style="margin: 0; width: 36px; height: 36px; line-height: 36px;" on:click={(e) => handleDelete(k.id, e)} title="削除">
              <i class="material-icons red-text text-lighten-1">delete</i>
            </button>
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
