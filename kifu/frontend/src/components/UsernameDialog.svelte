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
  <!-- Content click propagation stopped to avoid closing on inner click -->
  <div class="modal-content nm-modal" onclick={(e) => e.stopPropagation()} aria-hidden="true">
    <form onsubmit={handleSave}>
      
      <!-- Header -->
      <div class="modal-header">
        <span class="modal-title font-mincho">
          <i class="material-icons modal-title-icon">edit</i>
          ユーザー名の変更
        </span>
        <p class="modal-subtitle">
          添削コメントや公開ライブラリで表示されるお名前を変更できます。
        </p>
      </div>

      <!-- Error box style matched to Washi Clay theme colors -->
      {#if error}
        <div class="error-panel font-sans">
          <i class="material-icons error-icon">error_outline</i>
          <span>{error}</span>
        </div>
      {/if}

      <!-- Input Field wrapper -->
      <div class="input-wrapper">
        <div class="input-field" style="margin: 0;">
          <input 
            id="new-username" 
            type="text" 
            bind:value={newUsername} 
            required 
            class="nm-input" 
            placeholder="新しいユーザー名を入力" 
            style="margin-bottom: 0;" 
          />
          <label for="new-username" class="active font-sans">ユーザー名</label>
        </div>
      </div>

      <!-- Actions Footer -->
      <div class="modal-footer">
        <button type="button" class="cancel-btn" onclick={onClose} disabled={loading}>キャンセル</button>
        <button type="submit" class="save-btn" disabled={!newUsername.trim() || loading}>
          {#if loading}
            保存中...
          {:else}
            変更を保存
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
    background: rgba(37, 53, 48, 0.4); /* Consistent harmonized Sage Green translucent background */
    backdrop-filter: var(--wc-glass-blur);
    -webkit-backdrop-filter: var(--wc-glass-blur);
    z-index: 1000;
    display: flex;
    justify-content: center;
    align-items: center;
    padding: 16px;
  }

  .modal-content {
    box-sizing: border-box;
    width: 100%;
    max-width: 440px;
    background: var(--wc-surface) !important;
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-text) !important;
    box-shadow: 8px 8px 0px var(--wc-shadow-dark) !important;
    padding: 32px 28px !important;
    max-height: calc(100vh - 40px);
    overflow-y: auto;
  }

  .modal-header {
    margin-bottom: 24px;
    text-align: left;
  }

  .modal-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 800;
    color: var(--wc-text);
    font-size: 1.3rem;
    margin-bottom: 8px;
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .modal-title-icon {
    font-size: 1.5rem;
    color: var(--wc-accent);
  }

  .modal-subtitle {
    margin: 0;
    font-size: 0.82rem;
    color: var(--wc-text-muted);
    line-height: 1.6;
    font-family: 'DM Sans', sans-serif;
  }

  /* Harmonized Error Box */
  .error-panel {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(160, 50, 40, 0.06);
    border: 1.5px solid #9d2f2f;
    border-radius: 0px !important;
    padding: 10px 14px;
    margin-bottom: 18px;
    color: #9d2f2f;
    font-size: 0.85rem;
    line-height: 1.4;
  }

  .error-icon {
    font-size: 1.1rem;
    flex-shrink: 0;
  }

  .input-wrapper {
    margin-bottom: 8px;
  }

  /* Input Style - Vogue sharp borders */
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
  }

  .modal-footer {
    padding: 20px 0 0 0;
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    border-top: 1.5px solid var(--wc-border);
    margin-top: 24px;
  }

  /* Vogue sharp solid buttons */
  .save-btn {
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-text) !important;
    background: var(--wc-accent) !important;
    color: #FFFFFF !important;
    box-shadow: 3px 3px 0px var(--wc-text) !important;
    padding: 8px 24px !important;
    font-weight: 700;
    font-size: 0.88rem;
    font-family: 'DM Sans', sans-serif;
    cursor: pointer;
    transition: var(--wc-transition-fast);
  }

  .save-btn:hover:not(:disabled) {
    transform: translate(-1px, -1px);
    box-shadow: 4px 4px 0px var(--wc-text) !important;
    background: var(--wc-accent-hover) !important;
  }

  .save-btn:active:not(:disabled) {
    transform: translate(1px, 1px);
    box-shadow: 1px 1px 0px var(--wc-text) !important;
  }

  .save-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    box-shadow: none !important;
    transform: none !important;
  }

  .cancel-btn {
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-text) !important;
    background: var(--wc-surface) !important;
    color: var(--wc-text) !important;
    box-shadow: 3px 3px 0px var(--wc-text) !important;
    padding: 8px 24px;
    font-weight: 700;
    font-size: 0.88rem;
    font-family: 'DM Sans', sans-serif;
    cursor: pointer;
    transition: var(--wc-transition-fast);
  }

  .cancel-btn:hover:not(:disabled) {
    transform: translate(-1px, -1px);
    box-shadow: 4px 4px 0px var(--wc-text) !important;
    background: var(--wc-surface-alt) !important;
  }

  .cancel-btn:active:not(:disabled) {
    transform: translate(1px, 1px);
    box-shadow: 1px 1px 0px var(--wc-text) !important;
  }

  /* Typography / Fonts classes */
  .font-mincho {
    font-family: 'Shippori Mincho B1', serif;
  }

  .font-sans {
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
  }

  /* Animations & Responsive */
  .animate-fade-in {
    animation: fadeIn 0.25s ease-out;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @media only screen and (max-width: 400px) {
    .modal-footer {
      flex-direction: column-reverse;
      gap: 10px;
    }
    .modal-footer button {
      width: 100% !important;
      margin: 0 !important;
    }
  }
</style>
