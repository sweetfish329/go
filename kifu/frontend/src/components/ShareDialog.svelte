<script lang="ts">
  import QRCode from 'qrcode';
  import { auth } from '../lib/auth.svelte';

  interface Kifu {
    id: string;
    title: string;
    is_private?: boolean;
    share_token?: string;
  }

  let { kifu, onClose, onUpdate } = $props<{
    kifu: Kifu;
    onClose: () => void;
    onUpdate: (updatedKifu: Kifu) => void;
  }>();

  let loading = $state(false);
  let qrCodeSvg = $state("");
  let copySuccess = $state(false);
  let isPrivate = $state(false);

  $effect(() => {
    isPrivate = kifu.is_private !== false;
  });

  const getM = () => (window as any).M;

  // Compute share URL using the format /u/:userId/:kifuId
  const shareUrl = $derived.by(() => {
    const origin = window.location.origin;
    return `${origin}/u/${auth.userId}/${kifu.id}`;
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

  async function handleTogglePrivacy(checked: boolean) {
    loading = true;
    const nextIsPrivate = !checked;
    try {
      const res = await fetch(`/api/kifus/${kifu.id}/privacy`, {
        method: 'PUT',
        headers: auth.getHeaders(),
        body: JSON.stringify({
          is_private: nextIsPrivate
        })
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || "公開設定の更新に失敗しました。");
      }

      isPrivate = nextIsPrivate;
      onUpdate({ ...kifu, is_private: nextIsPrivate });

      const M = getM();
      if (M) {
        M.toast({ 
          html: nextIsPrivate ? '限定公開（リンクを知っている人のみ）に変更しました' : '一般公開（全員に公開）に変更しました！', 
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
  <div class="share-modal-content nm-modal" onclick={(e) => e.stopPropagation()} aria-hidden="true" style="padding: 24px !important;">
    <div>
      <span class="card-title" style="display: flex; align-items: center; gap: 8px; font-weight: 600; color: var(--wc-accent); font-size: 1.1rem; margin-bottom: 12px; font-family: 'Shippori Mincho B1', serif; letter-spacing: 0.04em;">
        <i class="material-icons">share</i>
        棋譜を共有する
      </span>
      <p style="margin-bottom: 20px; font-size: 0.88rem; color: var(--wc-text-muted); font-family: 'DM Sans', 'Noto Sans JP', sans-serif;">
        この棋譜にアクセスし、添削を受け取るための公開リンクとQRコードです。
      </p>
 
      <div class="share-active-box center-align">
        <div class="qr-container">
          {#if qrCodeSvg}
            <!-- eslint-disable-next-line svelte/no-at-html-tags -->
            {@html qrCodeSvg}
          {:else}
            <div class="valign-wrapper justify-center" style="height: 200px; display: flex; align-items: center;">
              <p class="text-muted" style="font-family: 'DM Sans', sans-serif;">QRコード生成中...</p>
            </div>
          {/if}
        </div>
 
        <div class="share-url-container valign-wrapper" style="margin-top: 20px; display: flex; align-items: center; gap: 10px;">
          <input type="text" readonly value={shareUrl} class="nm-input share-url-input" style="margin-bottom: 0; font-family: 'JetBrains Mono', monospace; font-size: 0.85rem;" />
          <button class="nm-btn-primary" onclick={handleCopy} style="height: 42px; width: 42px; display: flex; align-items: center; justify-content: center; padding: 0; min-width: 42px; border-radius: var(--wc-radius-sm); flex-shrink: 0;">
            <i class="material-icons" style="font-size: 1.2rem;">{copySuccess ? 'check' : 'content_copy'}</i>
          </button>
        </div>
 
        <!-- Privacy Toggle -->
        <div class="privacy-toggle-container nm-panel-inset left-align" style="margin-top: 20px; padding: 16px;">
          <div class="switch">
            <label style="font-weight: 500; display: flex; align-items: center; justify-content: space-between; cursor: pointer; gap: 8px; width: 100%; color: var(--wc-text); font-family: 'DM Sans', 'Noto Sans JP', sans-serif;">
              <span style="font-size: 0.85rem; color: var(--wc-text); font-weight: 600; text-align: left;">一般公開（ライブラリや検索に掲載）</span>
              <input type="checkbox" checked={!isPrivate} onchange={(e) => handleTogglePrivacy(e.currentTarget.checked)} disabled={loading}>
              <span class="lever brown lighten-3"></span>
            </label>
          </div>
          <p style="margin: 8px 0 0 0; font-size: 0.8rem; line-height: 1.4; color: var(--wc-text-muted);">
            {#if isPrivate}
              現在: <strong>限定公開</strong><br>リンクを知っている人だけが閲覧可能です。あなたの公開プロフィールや検索エンジンには掲載されません。
            {:else}
              現在: <strong>一般公開</strong><br>誰でも閲覧可能で、あなたの公開棋譜一覧に掲載され、Googleなどの検索エンジンにも登録されます。
            {/if}
          </p>
        </div>
      </div>
    </div>
 
    <div style="padding: 16px 0 0 0; display: flex; justify-content: flex-end; border-top: 1px solid var(--wc-border); margin-top: 20px;">
      <button class="nm-btn-flat" onclick={onClose}>閉じる</button>
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
    background: rgba(163, 177, 198, 0.4);
    backdrop-filter: blur(5px);
    z-index: 1000;
    display: flex;
    justify-content: center;
    align-items: center;
  }
  .qr-container {
    display: inline-block;
    padding: 12px;
    background: white;
    border-radius: var(--nm-radius-md);
    box-shadow: var(--nm-shadow-inset);
    border: var(--nm-border-dark);
  }
  .share-url-container {
    display: flex;
    width: 100%;
  }
  .share-url-input {
    flex-grow: 1;
    margin: 0 !important;
  }
  
  /* Mobile responsive adjustments */
  @media only screen and (max-width: 480px) {
    .share-url-container {
      flex-direction: column;
      gap: 10px;
    }
    .share-url-input {
      width: 100% !important;
    }
    .share-url-container button {
      width: 100% !important;
      height: 40px !important;
    }
    .privacy-toggle-container label {
      flex-direction: column !important;
      align-items: flex-start !important;
      gap: 8px !important;
    }
    .privacy-toggle-container .lever {
      margin-left: 0 !important;
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
