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
  let isPrivate = $state(kifu.is_private !== false);

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
  <div class="share-modal-content card" onclick={(e) => e.stopPropagation()} aria-hidden="true">
    <div class="card-content">
      <span class="card-title brown-text text-darken-3 d-flex align-center" style="display: flex; align-items: center; gap: 8px; font-weight: 500;">
        <i class="material-icons">share</i>
        棋譜を共有する
      </span>
      <p class="grey-text text-darken-1" style="margin-bottom: 20px;">
        この棋譜にアクセスし、添削を受け取るための公開リンクとQRコードです。
      </p>

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

        <!-- Privacy Toggle -->
        <div class="privacy-toggle-container left-align" style="margin-top: 25px; padding: 15px; background-color: #f5f5f5; border-radius: 8px; border: 1px dashed #ccc;">
          <div class="switch">
            <label class="black-text" style="font-weight: 500; display: flex; align-items: center; justify-content: space-between; cursor: pointer;">
              <span>一般公開（公開ライブラリや検索エンジンに掲載）</span>
              <input type="checkbox" checked={!isPrivate} onchange={(e) => handleTogglePrivacy(e.currentTarget.checked)} disabled={loading}>
              <span class="lever brown lighten-3"></span>
            </label>
          </div>
          <p class="grey-text text-darken-1" style="margin: 8px 0 0 0; font-size: 0.8rem; line-height: 1.4;">
            {#if isPrivate}
              現在: <strong>限定公開</strong><br>リンクを知っている人だけが閲覧可能です。あなたの公開プロフィールや検索エンジンには掲載されません。
            {:else}
              現在: <strong>一般公開</strong><br>誰でも閲覧可能で、あなたの公開棋譜一覧に掲載され、Googleなどの検索エンジンにも登録されます。
            {/if}
          </p>
        </div>
      </div>
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
