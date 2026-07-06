<script lang="ts">
  import { auth } from '../lib/auth.svelte';

  let { onClose, onSuccess } = $props<{
    onClose: () => void;
    onSuccess: (newUsername: string) => void;
  }>();

  let newUsername = $state(auth.username || "");
  let error = $state<string | null>(null);
  let loading = $state(false);

  const getM = () => (window as any).M;

  async function handleSave(e: Event) {
    e.preventDefault();
    if (!newUsername.trim()) {
      error = "ユーザー名を入力してください。";
      return;
    }

    loading = true;
    error = null;

    try {
      const res = await fetch('/api/auth/username', {
        method: 'PUT',
        headers: auth.getHeaders(),
        body: JSON.stringify({
          username: newUsername.trim()
        })
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || "ユーザー名の変更に失敗しました。");
      }

      // Update auth store local state
      auth.setLogin(auth.token || "", data.username, data.id);

      const M = getM();
      if (M) {
        M.toast({ html: 'ユーザー名を変更しました！', classes: 'green darken-1' });
      }

      onSuccess(data.username);
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<div class="modal-backdrop animate-fade-in" onclick={onClose} aria-hidden="true">
  <div class="modal-content card" onclick={(e) => e.stopPropagation()} aria-hidden="true">
    <form onsubmit={handleSave}>
      <div class="card-content">
        <span class="card-title brown-text text-darken-3 d-flex align-center" style="display: flex; align-items: center; gap: 8px; font-weight: 500;">
          <i class="material-icons">edit</i>
          ユーザー名の変更
        </span>
        <p class="grey-text text-darken-1" style="margin-bottom: 20px;">
          指導碁や添削コメントの投稿時に表示される名前を変更できます。
        </p>

        {#if error}
          <div class="card-panel red lighten-4 red-text text-darken-4 valign-wrapper" style="padding: 10px; margin-bottom: 15px;">
            <i class="material-icons left">error</i>
            <span>{error}</span>
          </div>
        {/if}

        <div class="input-field">
          <input id="new-username" type="text" bind:value={newUsername} required class="validate" />
          <label for="new-username" class="active">新しいユーザー名</label>
        </div>
      </div>

      <div class="card-action right-align" style="background-color: #fafafa; padding: 15px 24px;">
        <button type="button" class="btn-flat waves-effect" onclick={onClose} disabled={loading}>キャンセル</button>
        <button type="submit" class="btn waves-effect waves-light brown" disabled={!newUsername.trim() || loading}>
          {#if loading}
            保存中...
          {:else}
            保存
          {/if}
        </button>
      </div>
    </form>
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(0, 0, 0, 0.4);
    backdrop-filter: blur(4px);
    z-index: 1000;
    display: flex;
    justify-content: center;
    align-items: center;
  }
  .modal-content {
    width: 92%;
    max-width: 400px;
    border-radius: 12px;
    overflow: hidden;
    margin: 10px;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  }
  
  /* Mobile responsive adjustments */
  @media only screen and (max-width: 400px) {
    :global(.card-action) {
      display: flex;
      flex-direction: column-reverse;
      gap: 10px;
    }
    :global(.card-action) button {
      width: 100%;
      margin: 0 !important;
      height: 40px !important;
    }
  }

  .animate-fade-in {
    animation: fadeIn 0.25s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
</style>
