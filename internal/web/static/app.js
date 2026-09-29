// PM Jira client script.
// Board drag-and-drop is implemented with native HTML5 DnD + htmx.ajax for the
// status swap. Listeners are delegated on `document` so they survive the HTMX
// board re-render after each move.

// versionView switches the versions page between grouped/flat/roadmap views.
window.versionView = function (view) {
  if (view === 'group' || !view) {
    location.href = '/versions';
  } else {
    location.href = '/versions?view=' + encodeURIComponent(view);
  }
};

(function () {
  var draggingCard = null;

  function closest(el, sel) {
    return el && el.closest ? el.closest(sel) : null;
  }

  document.addEventListener('dragstart', function (e) {
    var card = closest(e.target, '.board-card');
    if (!card) return;
    draggingCard = card;
    card.classList.add('dragging');
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', card.getAttribute('data-issue') || '');
    }
  });

  document.addEventListener('dragend', function () {
    if (draggingCard) draggingCard.classList.remove('dragging');
    draggingCard = null;
    document.querySelectorAll('.board-col.drop-target').forEach(function (c) {
      c.classList.remove('drop-target');
    });
  });

  document.addEventListener('dragover', function (e) {
    if (!closest(e.target, '#board')) return;
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
    var col = closest(e.target, '.board-col');
    document.querySelectorAll('.board-col.drop-target').forEach(function (c) {
      if (c !== col) c.classList.remove('drop-target');
    });
    if (col) col.classList.add('drop-target');
  });

  document.addEventListener('drop', function (e) {
    if (!closest(e.target, '#board')) return;
    e.preventDefault();
    var col = closest(e.target, '.board-col');
    var card = draggingCard;
    if (!col || !card) return;
    var id = card.getAttribute('data-issue');
    var status = col.getAttribute('data-status');
    if (!id || !status) return;
    if (window.htmx) {
      htmx.ajax('POST', '/issues/' + id + '/status', {
        target: '#board',
        swap: 'outerHTML',
        values: { status: status }
      });
    }
  });
})();

// Auto-dismiss the flash banner after a few seconds.
(function () {
  function dismiss() {
    var f = document.getElementById('flash');
    if (f) {
      f.style.transition = 'opacity .4s';
      f.style.opacity = '0';
      setTimeout(function () { f.remove(); }, 500);
    }
  }
  setTimeout(dismiss, 4000);
})();

// Image lightbox for attachment thumbnails. Global so inline onclick can call it.
window.openLightbox = function (src) {
  var box = document.createElement('div');
  box.id = 'lightbox';
  var img = document.createElement('img');
  img.src = src;
  img.alt = '';
  var close = document.createElement('button');
  close.className = 'lb-close';
  close.type = 'button';
  close.textContent = '×';
  close.setAttribute('aria-label', '关闭');
  function closeBox() { if (box.parentNode) box.parentNode.removeChild(box); }
  close.onclick = function (e) { e.stopPropagation(); closeBox(); };
  box.onclick = function (e) { if (e.target === box) closeBox(); };
  box.appendChild(close);
  box.appendChild(img);
  document.body.appendChild(box);
  document.addEventListener('keydown', function onKey(e) {
    if (e.key === 'Escape') { closeBox(); document.removeEventListener('keydown', onKey); }
  });
};

// Markdown editor tabs use the same server-side renderer as the saved detail
// pages, so preview and final output always follow identical safety rules.
(function () {
  document.querySelectorAll('[data-markdown-editor]').forEach(function (editor) {
    var textarea = editor.querySelector('textarea');
    var preview = editor.querySelector('[data-markdown-preview]');
    var tabs = editor.querySelectorAll('[data-markdown-tab]');
    var panels = editor.querySelectorAll('[data-markdown-panel]');
    var controller = null;
    var timer = null;
    var requestID = 0;

    if (!textarea || !preview) return;

    function selectTab(name) {
      tabs.forEach(function (tab) {
        var active = tab.getAttribute('data-markdown-tab') === name;
        tab.classList.toggle('active', active);
        tab.setAttribute('aria-selected', active ? 'true' : 'false');
        tab.tabIndex = active ? 0 : -1;
      });
      panels.forEach(function (panel) {
        panel.hidden = panel.getAttribute('data-markdown-panel') !== name;
      });
      if (name === 'preview') renderPreview();
    }

    function renderPreview() {
      if (controller) controller.abort();
      controller = typeof AbortController !== 'undefined' ? new AbortController() : null;
      var currentRequest = ++requestID;
      preview.classList.remove('markdown-preview-error');
      preview.innerHTML = '<div class="muted">正在生成预览…</div>';

      var body = new URLSearchParams();
      body.set('markdown', textarea.value);
      var options = {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8' },
        body: body.toString()
      };
      if (controller) options.signal = controller.signal;

      fetch('/markdown/preview', options).then(function (response) {
        if (!response.ok) throw new Error('HTTP ' + response.status);
        return response.text();
      }).then(function (html) {
        if (currentRequest === requestID) preview.innerHTML = html;
      }).catch(function (err) {
        if (err.name === 'AbortError' || currentRequest !== requestID) return;
        preview.textContent = '预览生成失败，请稍后重试。';
        preview.classList.add('markdown-preview-error');
      });
    }

    tabs.forEach(function (tab) {
      tab.addEventListener('click', function () {
        preview.classList.remove('markdown-preview-error');
        selectTab(tab.getAttribute('data-markdown-tab'));
      });
      tab.addEventListener('keydown', function (e) {
        if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
        e.preventDefault();
        var next = tab.getAttribute('data-markdown-tab') === 'edit' ? 'preview' : 'edit';
        selectTab(next);
        editor.querySelector('[data-markdown-tab="' + next + '"]').focus();
      });
    });

    textarea.addEventListener('input', function () {
      var previewPanel = editor.querySelector('[data-markdown-panel="preview"]');
      if (!previewPanel || previewPanel.hidden) return;
      clearTimeout(timer);
      timer = setTimeout(renderPreview, 300);
    });
  });
})();
