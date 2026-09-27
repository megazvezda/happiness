const go = new Go();
const input = document.querySelector('#pdf-input');
const dropZone = document.querySelector('#drop-zone');
const status = document.querySelector('#status');
const downloadLink = document.querySelector('#download-link');

let wasmReady = false;

function setStatus(message, state = '') {
  status.className = `status ${state}`.trim();
  status.lastChild.textContent = ` ${message}`;
}

function showFile(file) {
  if (!file) {
    downloadLink.classList.add('disabled');
    downloadLink.setAttribute('aria-disabled', 'true');
    return;
  }

  if (file.type !== 'application/pdf' && !file.name.toLowerCase().endsWith('.pdf')) {
    input.value = '';
    downloadLink.classList.add('disabled');
    downloadLink.setAttribute('aria-disabled', 'true');
    setStatus('please choose a .pdf file', 'error');
    return;
  }

  downloadLink.classList.toggle('disabled', !wasmReady);
  downloadLink.setAttribute('aria-disabled', String(!wasmReady));
  setStatus(wasmReady ? 'wasm loaded, ready to convert' : 'Loading converter', wasmReady ? 'ready' : '');
}

WebAssembly.instantiateStreaming(fetch('app.wasm'), go.importObject)
  .then((result) => {
    go.run(result.instance);
    wasmReady = true;
    setStatus(input.files[0] ? 'Ready' : `choose a pdf and we'll get ready`, 'ready');
    showFile(input.files[0]);
  })
  .catch((error) => {
    setStatus(`WASM failed to load: ${error.message}`, 'error');
  });

input.addEventListener('change', () => showFile(input.files[0]));

['dragenter', 'dragover'].forEach((eventName) => {
  dropZone.addEventListener(eventName, (event) => {
    event.preventDefault();
    dropZone.classList.add('dragging');
  });
});

['dragleave', 'drop'].forEach((eventName) => {
  dropZone.addEventListener(eventName, (event) => {
    event.preventDefault();
    dropZone.classList.remove('dragging');
  });
});

dropZone.addEventListener('drop', (event) => {
  const file = event.dataTransfer.files[0];
  if (!file) return;

  const transfer = new DataTransfer();
  transfer.items.add(file);
  input.files = transfer.files;
  showFile(file);
});

dropZone.addEventListener('keydown', (event) => {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault();
    input.click();
  }
});

downloadLink.addEventListener('click', async (event) => {
  event.preventDefault();
  const file = input.files[0];
  if (!file || !wasmReady || downloadLink.classList.contains('disabled')) return;

  downloadLink.classList.add('disabled');
  downloadLink.setAttribute('aria-disabled', 'true');
  setStatus('Converting');

  try {
    const pdfBytes = new Uint8Array(await file.arrayBuffer());
    const result = window.convertTimetable(pdfBytes);

    if (typeof result === 'object' && result.error) {
      throw new Error(result.error);
    }

    const blob = new Blob([result], { type: 'text/calendar;charset=utf-8' });
    const downloadUrl = URL.createObjectURL(blob);
    const downloadLink = document.createElement('a');
    downloadLink.href = downloadUrl;
    downloadLink.download = 'timetable.ics';
    downloadLink.click();
    URL.revokeObjectURL(downloadUrl);
    setStatus('Conversion complete', 'ready');
  } catch (error) {
    setStatus(`Conversion failed: ${error.message}`, 'error');
  } finally {
    downloadLink.classList.remove('disabled');
    downloadLink.setAttribute('aria-disabled', 'false');
  }
});
