<script lang="ts">
  import QRCode from 'qrcode';
  import { auth } from '../lib/auth.svelte';

  interface Kifu {
    id: string;
    title: string;
    share_token?: string;
    share_expires_at?: string;
  }

  let { kifu, onClose, onUpdate } = $props<{
    kifu: Kifu;
    onClose: () => void;
    onUpdate: (updatedKifu: Kifu) => void;
  }>();

  let expiresInDays = $state(0); // 0 means infinite
  let loading = $state(false);
  let qrCodeSvg = $state("");
  let copySuccess = $state(false);

  const getM = () => (window as any).M;

  // Compute share URL
  const shareUrl = $derived.by(() => {
    if (!kifu.share_token) return "";
    const origin = window.location.origin;
    return `${origin}/?share=${kifu.share_token}`;
  });

  // Generate QR code when shareUrl changes
  $effect(() => {
    if (shareUrl) {
      QRCode.toString(shareUrl, { type: 'svg', width: 200, margin: 2 }, (err, string) => {
        if (err) {
          console.error(err);
          qrCodeSvg = "";
        } else {
          qrCodeSvg = string;
        }
      });
    } else {
      qrCodeSvg = "";
    }
  });

  async function handleShare(disable = false) {
    loading = true;
    try {
      const res = await fetch(`/api/kifus/${kifu.id}/share`, {
        method: 'POST',
        headers: auth.getHeaders(),
        body: JSON.stringify({
          expires_in_days: disable ? null : (expiresInDays > 0 ? expiresInDays : null),
          disable: disable
        })
      });

      const updated = await res.json();
      if (!res.ok) {
        throw new Error(updated.error || "共有設定の更新に失敗しました。");
      }

      onUpdate(updated);

      const M = getM();
      if (M) {
        M.toast({ 
          html: disable ? '共有リンクを無効化しました' : '共有リンクを生成しました！', 
          classes: 'green darken-1' 
        });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + err.message, classes: 'red darken-1' });
      }
    } finally {
      loading = false;
    }
  }

  async function handleCopy() {
    if (!shareUrl) return;
    try {
      await navigator.clipboard.writeText(shareUrl);
      copySuccess = true;
      setTimeout(() => { copySuccess = false; }, 2000);
      const M = getM();
      if (M) {
        M.toast({ html: 'リンクをクリップボードにコピーしました！', classes: 'grey darken-3' });
      }
    } catch (err) {
      console.error("Failed to copy", err);
    }
  }
</script>

<div class="share-modal-backdrop animate-fade-in" onclick={onClose} aria-hidden="true">
  <div class="share-modal-content card" onclick={(e) => e.stopPropagation()} aria-hidden="true">
    <div class="card-content">
      <span class="card-title brown-text text-darken-3 d-flex align-center" style="display: flex; align-items: center; gap: 8px; font-weight: 500;">
        <i class="material-icons">share</i>
        棋譜を共有する
      </span>
      <p class="grey-text text-darken-1" style="margin-bottom: 20px;">
        この棋譜にアクセスし、添削を受け取るための公開リンクとQRコードを作成します。
      </p>

      {#if kifu.share_token}
        <!-- Active Share State -->
        <div class="share-active-box center-align">
          <div class="qr-container z-depth-1">
            {#if qrCodeSvg}
              <!-- eslint-disable-next-line svelte/no-at-html-tags -->
              {@html qrCodeSvg}
            {:else}
              <div class="valign-wrapper justify-center" style="height: 200px; display: flex; align-items: center;">
                <p class="grey-text">QRコード生成中...</p>
              </div>
            {/if}
          </div>

          <div class="share-url-container valign-wrapper" style="margin-top: 20px; display: flex; align-items: center;">
            <input type="text" readonly value={shareUrl} class="share-url-input" />
            <button class="btn brown darken-1 waves-effect waves-light" onclick={handleCopy} style="height: 36px; display: flex; align-items: center; justify-content: center; padding: 0 12px; margin-left: 8px;">
              <i class="material-icons" style="font-size: 1.2rem;">{copySuccess ? 'check' : 'content_copy'}</i>
            </button>
          </div>

          <div class="expires-info grey-text text-darken-1" style="margin-top: 15px; font-size: 0.9rem;">
            {#if kifu.share_expires_at}
              公開期限: {new Date(kifu.share_expires_at).toLocaleString('ja-JP')}
            {:else}
              公開期限: 無期限
            {/if}
          </div>

          <div class="center-align" style="margin-top: 30px;">
            <button class="btn-flat waves-effect waves-red red-text" onclick={() => handleShare(true)} disabled={loading}>
              <i class="material-icons left">link_off</i>共有リンクを無効化
            </button>
          </div>
        </div>
      {:else}
        <!-- Inactive Share State (Setup) -->
        <div class="share-setup-box">
          <div class="input-field">
            <select id="expires-select" bind:value={expiresInDays} class="browser-default" style="border: 1px solid #ccc; border-radius: 4px; padding: 10px; width: 100%; display: block; height: auto;">
              <option value={0}>無期限 (デフォルト)</option>
              <option value={1}>1日間有効</option>
              <option value={7}>7日間有効</option>
              <option value={30}>30日間有効</option>
            </select>
            <label for="expires-select" class="active brown-text text-darken-3" style="font-size: 0.9rem; position: static; display: block; margin-bottom: 8px;">公開期限を設定</label>
          </div>

          <div class="center-align" style="margin-top: 40px;">
            <button class="btn btn-large waves-effect waves-light brown darken-2" onclick={() => handleShare(false)} disabled={loading} style="border-radius: 6px; width: 100%;">
              <i class="material-icons left">link</i>共有用リンク・QRコードを作成
            </button>
          </div>
        </div>
      {/if}
    </div>

    <div class="card-action right-align" style="background-color: #fafafa; padding: 15px 24px;">
      <button class="btn-flat waves-effect" onclick={onClose}>閉じる</button>
    </div>
  </div>
</div>

<style>
  .share-modal-backdrop {
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
  .share-modal-content {
    width: 90%;
    max-width: 500px;
    border-radius: 12px;
    overflow: hidden;
    margin: 0;
  }
  .qr-container {
    display: inline-block;
    padding: 10px;
    background: white;
    border-radius: 8px;
    border: 1px solid #eee;
  }
  .share-url-container {
    display: flex;
    gap: 8px;
    width: 100%;
  }
  .share-url-input {
    flex-grow: 1;
    border: 1px solid #ccc !important;
    border-radius: 4px !important;
    padding: 0 10px !important;
    height: 36px !important;
    box-sizing: border-box !important;
    margin: 0 !important;
    background-color: #f5f5f5;
  }
  .animate-fade-in {
    animation: fadeIn 0.25s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
</style>
