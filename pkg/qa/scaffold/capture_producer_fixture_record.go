package scaffold

// captureFixtureTailJS continues captureFixtureHeadJS: request recording, the
// shard writer, and the exported fixture. Concatenated verbatim, so the emitted
// file is byte-identical to the single-literal form.
const captureFixtureTailJS = `async function recordRequest(policy, state, request, failed) {
  try {
    const method = String(request.method() || '').toUpperCase();
    const ref = urlRef(policy, request.url());
    if (!HTTP_METHOD.test(method) || !ref) return;
    const response = failed ? null : await request.response();
    const sizes = await request.sizes().catch(function () { return null; });
    const timing = request.timing() || {};
    if (state.network.entries.length >= MAX.network) return;
    state.network.entries.push({
      method: method,
      url_ref: ref,
      status: response ? response.status() : 0,
      resource_type: String(request.resourceType() || ''),
      duration_ms: timing.responseEnd > 0 ? Math.round(timing.responseEnd) : 0,
      bytes: sizes && sizes.responseBodySize > 0 ? sizes.responseBodySize : 0,
    });
  } catch (error) {
    pushError(state, 'network', error);
  }
}

function targetRef(policy, api, arg) {
  if (api === 'goto') return urlRef(policy, arg);
  if (typeof arg !== 'string') return '';
  return arg.length > MAX.target ? arg.slice(0, MAX.target) : arg;
}

// screenRefOf labels a step with the screen it covered, which is the only input
// the gui.screen_matrix oracle counts. An explicit annotation wins so a spec can
// state its own coverage; otherwise the first navigation is used, which lets
// hand-authored specs satisfy a path-keyed matrix with no edit. The 'origin:<n>'
// prefix is stripped because the matrix declares origin-relative paths.
function screenRefOf(testInfo, state) {
  const declared = (testInfo.annotations || []).filter(function (note) {
    return note && String(note.type) === 'autopus-screen' && plain(note.description);
  });
  if (declared.length) return plain(declared[0].description).slice(0, MAX.ref);
  for (let index = 0; index < state.actions.length; index += 1) {
    const action = state.actions[index];
    if (action.api !== 'goto' || !action.target_ref) continue;
    const stripped = String(action.target_ref).replace(/^origin:\d+/, '');
    return (stripped || '/').slice(0, MAX.ref);
  }
  return '';
}

function wrapActions(page, policy, state) {
  WRAPPED_APIS.forEach(function (api) {
    const original = page[api];
    if (typeof original !== 'function') return;
    page[api] = function () {
      const args = Array.prototype.slice.call(arguments);
      const started = Date.now();
      const record = function () {
        if (state.actions.length >= MAX.actions) return;
        const action = { api: api, duration_ms: Date.now() - started };
        const ref = targetRef(policy, api, args[0]);
        if (ref) action.target_ref = ref;
        state.actions.push(action);
      };
      return original.apply(page, args).then(
        function (value) { record(); return value; },
        function (error) { record(); throw error; });
    };
  });
}

function mediaRef(dir, rel, width, height) {
  const body = fs.readFileSync(path.join(dir, rel));
  if (!body.length) return null;
  const ref = {
    ref: rel,
    digest: 'sha256:' + crypto.createHash('sha256').update(body).digest('hex'),
    bytes: body.length,
    retention: 'local_only',
  };
  if (width > 0 && height > 0) { ref.width = width; ref.height = height; }
  return ref;
}

// slugFor is both the step_id and the media basename. The hash suffix keeps two
// tests with the same truncated title from colliding on one step_id.
function slugFor(testInfo) {
  const raw = (testInfo.titlePath || []).concat([String(testInfo.repeatEachIndex || 0), String(testInfo.retry || 0)]).join('-');
  const suffix = crypto.createHash('sha256').update(raw).digest('hex').slice(0, 8);
  const slug = raw.replace(/[^A-Za-z0-9._-]+/g, '-').replace(/^-+/, '').replace(/-+$/, '').slice(0, 90);
  return (slug || 'step') + '-' + suffix;
}

function specRef(testInfo) {
  const file = testInfo.file || '';
  const rel = path.relative((testInfo.config && testInfo.config.rootDir) || process.cwd(), file);
  if (!rel || path.isAbsolute(rel) || rel.indexOf('..') === 0) return path.basename(file) || 'spec';
  return rel.split(path.sep).join('/');
}

function failureSummary(testInfo, status) {
  const parts = (testInfo.errors || []).map(errorText).filter(Boolean);
  if (!parts.length && status !== 'passed' && status !== 'skipped') parts.push('test ' + status + ' without a reported error');
  return parts.join(' | ').slice(0, MAX.text);
}

function shouldShoot(policy, status) {
  if (!policy.streams.includes('screenshot') || status === 'skipped') return false;
  if (policy.screenshot === 'per-step') return true;
  return policy.screenshot === 'on-failure' && (status === 'failed' || status === 'blocked');
}

async function finish(page, testInfo, policy, state, startedAt) {
  const slug = slugFor(testInfo);
  const status = STATUS[testInfo.status] || 'blocked';
  await guard(state, 'network', function () { return Promise.all(state.pending); });
  const shard = {
    step_id: slug,
    title: plain(testInfo.title).slice(0, MAX.text),
    status: status,
    started_at: startedAt,
    ended_at: new Date().toISOString(),
    duration_ms: Math.max(0, Math.round(testInfo.duration || 0)),
    spec_ref: specRef(testInfo),
    actions: state.actions,
  };
  const screenRef = screenRefOf(testInfo, state);
  if (screenRef) shard.screen_ref = screenRef;
  if (policy.streams.includes('console')) shard.console = state.console;
  if (policy.streams.includes('network')) shard.network = state.network;
  if (shouldShoot(policy, status)) {
    const rel = 'screenshots/' + slug + '.png';
    const ref = await guard(state, 'screenshot', async function () {
      fs.mkdirSync(path.join(policy.dir, 'screenshots'), { recursive: true });
      await page.screenshot({ path: path.join(policy.dir, rel) });
      const viewport = page.viewportSize() || { width: 0, height: 0 };
      return mediaRef(policy.dir, rel, viewport.width, viewport.height);
    });
    if (ref) shard.screenshot = ref;
  }
  if (policy.streams.includes('trace')) {
    await guard(state, 'trace_stop', async function () {
      fs.mkdirSync(path.join(policy.dir, 'traces'), { recursive: true });
      await page.context().tracing.stop({ path: path.join(policy.dir, 'traces', slug + '.zip') });
    });
  }
  const summary = failureSummary(testInfo, status);
  if (summary) shard.failure_summary = summary;
  if (state.errors.length) shard.capture_error = state.errors.join(' | ').slice(0, MAX.text);
  const target = path.join(policy.dir, 'shards', slug + '.json');
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.writeFileSync(target, JSON.stringify(shard, null, 2) + '\n');
}

const test = base.test.extend({
  autopusCapture: [
    async function autopusCapture({ page }, use, testInfo) {
      const policy = safePolicy();
      if (!policy || policy.mode === 'off') {
        // Outside a QAMESH run this fixture adds nothing: 'playwright test'
        // must behave exactly as it did before capture existed.
        await use(null);
        return;
      }
      const state = {
        threshold: policy.threshold,
        console: { errors: 0, warnings: 0, infos: 0, messages: [] },
        network: { requests: 0, failures: 0, entries: [] },
        actions: [], pending: [], errors: [],
      };
      if (policy.streams.includes('trace')) {
        await guard(state, 'trace_start', function () {
          return page.context().tracing.start({ screenshots: true, snapshots: true });
        });
      }
      attachListeners(page, policy, state);
      wrapActions(page, policy, state);
      const startedAt = new Date().toISOString();
      await use(state);
      await guard(state, 'finish', function () { return finish(page, testInfo, policy, state, startedAt); });
    },
    { auto: true },
  ],
});

module.exports = { test: test, expect: base.expect };
`
