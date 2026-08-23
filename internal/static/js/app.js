// Client-side interaction helpers (Vanilla JS, Zero Dependencies)

// Single-Click MP3 Playback Controller
function playAudio(streamUrl, filename) {
  const bar = document.getElementById('globalAudioBar');
  const player = document.getElementById('globalAudioPlayer');
  const trackName = document.getElementById('audioTrackName');

  if (!bar || !player) return;

  trackName.textContent = filename || 'Audio Track';
  player.src = streamUrl;
  bar.classList.add('active');
  player.play().catch(err => {
    console.warn('Autoplay prevented by browser:', err);
  });
}

function closeAudioPlayer() {
  const bar = document.getElementById('globalAudioBar');
  const player = document.getElementById('globalAudioPlayer');
  if (player) {
    player.pause();
    player.src = '';
  }
  if (bar) {
    bar.classList.remove('active');
  }
}

// Modal Dialog Helpers
function openDialog(id) {
  const dlg = document.getElementById(id);
  if (dlg && typeof dlg.showModal === 'function') {
    dlg.showModal();
  }
}

function closeDialog(id) {
  const dlg = document.getElementById(id);
  if (dlg && typeof dlg.close === 'function') {
    dlg.close();
  }
}

// Share Modal Setup
function openShareModal(itemId, itemType, shareToken) {
  const dlg = document.getElementById('shareModal');
  const inputItemId = document.getElementById('shareItemId');
  const inputItemType = document.getElementById('shareItemType');
  const shareLinkSection = document.getElementById('existingShareSection');
  const shareLinkInput = document.getElementById('shareLinkInput');

  if (inputItemId) inputItemId.value = itemId;
  if (inputItemType) inputItemType.value = itemType;

  if (shareToken && shareToken !== "") {
    const fullUrl = window.location.origin + '/share/' + shareToken;
    if (shareLinkInput) shareLinkInput.value = fullUrl;
    if (shareLinkSection) shareLinkSection.style.display = 'block';
  } else {
    if (shareLinkSection) shareLinkSection.style.display = 'none';
  }

  openDialog('shareModal');
}

// Copy to Clipboard Utility
function copyToClipboard(text, btnElement) {
  if (!navigator.clipboard) {
    // Fallback
    const textArea = document.createElement("textarea");
    textArea.value = text;
    document.body.appendChild(textArea);
    textArea.select();
    try {
      document.execCommand('copy');
      indicateCopySuccess(btnElement);
    } catch (err) {
      console.error('Fallback copy failed', err);
    }
    document.body.removeChild(textArea);
    return;
  }

  navigator.clipboard.writeText(text).then(() => {
    indicateCopySuccess(btnElement);
  }).catch(err => {
    console.error('Copy failed:', err);
  });
}

function indicateCopySuccess(btn) {
  if (!btn) return;
  const originalText = btn.textContent;
  btn.textContent = 'Copied!';
  btn.classList.add('btn-primary');
  setTimeout(() => {
    btn.textContent = originalText;
    btn.classList.remove('btn-primary');
  }, 2000);
}
