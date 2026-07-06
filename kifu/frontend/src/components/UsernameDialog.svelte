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
  <div class="modal-content nm-modal" onclick={(e) => e.stopPropagation()} aria-hidden="true" style="padding: 24px !important;">
    <form onsubmit={handleSave}>
      <div>
        <span class="card-title d-flex align-center" style="display: flex; align-items: center; gap: 8px; font-weight: 600; color: var(--nm-accent); font-size: 1.2rem; margin-bottom: 12px;">
          <i class="material-icons">edit</i>
          ユーザー名の変更
        </span>
        <p style="margin-bottom: 24px; font-size: 0.9rem; color: var(--nm-text-muted);">
          指導碁や添削コメントの投稿時に表示される名前を変更できます。
        </p>

        {#if error}
          <div class="card-panel red lighten-4 red-text text-darken-4 valign-wrapper" style="padding: 10px; margin-bottom: 15px; border-radius: 8px; border: 1px solid rgba(239,83,80,0.35);">
            <i class="material-icons left">error</i>
            <span>{error}</span>
          </div>
        {/if}

        <div class="input-field" style="margin-top: 0;">
          <input id="new-username" type="text" bind:value={newUsername} required class="nm-input" style="margin-bottom: 0;" />
          <label for="new-username" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem;">新しいユーザー名</label>
        </div>
      </div>

      <div style="padding: 20px 0 0 0; display: flex; justify-content: flex-end; gap: 12px; border-top: 1px solid rgba(163, 177, 198, 0.2); margin-top: 20px;">
        <button type="button" class="nm-btn-flat" onclick={onClose} disabled={loading}>キャンセル</button>
        <button type="submit" class="nm-btn-primary" disabled={!newUsername.trim() || loading}>
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
    background: rgba(163, 177, 198, 0.4);
    backdrop-filter: blur(5px);
    z-index: 1000;
    display: flex;
    justify-content: center;
    align-items: center;
  }
  
  /* Mobile responsive adjustments */
  @media only screen and (max-width: 400px) {
    div[style*="justify-content: flex-end"] {
      display: flex;
      flex-direction: column-reverse;
      gap: 10px;
    }
    div[style*="justify-content: flex-end"] button {
      width: 100%;
      margin: 0 !important;
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
