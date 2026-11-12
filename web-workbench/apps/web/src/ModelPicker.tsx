import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { createPortal } from 'react-dom';
import type { ChatModel } from '../../../packages/contracts/src/chat.ts';
import './ModelPicker.css';

type ModelId = Pick<ChatModel, 'providerID' | 'id' | 'variant'>;
type Props = {
  value?: ModelId;
  disabled?: boolean;
  allowDefault?: boolean;
  loadModels: () => Promise<ChatModel[]>;
  onSelect: (model: ChatModel | undefined) => void | Promise<void>;
  onApplied?: () => void;
};

const identity = (model?: ModelId) => model ? JSON.stringify([model.providerID, model.id]) : '';
const modelPath = (model: ModelId) => model.providerID + '/' + model.id;

function readHistory(key: string): string[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(key) ?? '[]');
    return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
  } catch { return []; }
}

function saveHistory(key: string, value: string[]) {
  try { localStorage.setItem(key, JSON.stringify(value)); } catch { /* Storage can be disabled in private browsing. */ }
}

const recentKey = 'harness.models.recent';
const favoriteKey = 'harness.models.favorites';
type Group = { title: string; models: (ChatModel | undefined)[] };

function buildGroups(catalog: ChatModel[], query: string, favorites: string[], recent: string[], allowDefault: boolean): Group[] {
  const groups: Group[] = [];
  const words = query.trim().toLowerCase().split(/\s+/);
  const searching = Boolean(words[0]);
  const matches = (name: string) => words.every(word => name.toLowerCase().includes(word));
  if (allowDefault && matches('OpenCode 默认模型')) groups.push({title: '', models: [undefined]});
  const sorted = [...catalog].sort((a, b) => a.providerID.localeCompare(b.providerID) || a.name.localeCompare(b.name) || a.id.localeCompare(b.id));
  if (searching) {
    groups.push({title: '搜索结果', models: sorted.filter(m => matches(m.name + ' ' + modelPath(m)))});
  } else {
    const byId = new Map(sorted.map(m => [identity(m), m]));
    const fromHistory = (ids: string[]) => ids.flatMap(id => byId.get(id) ?? []);
    groups.push({title: '收藏', models: fromHistory(favorites)}, {title: '最近使用', models: fromHistory(recent).slice(0, 5)});
    const providers = new Map<string, ChatModel[]>();
    for (const model of sorted) {
      if (!providers.has(model.providerID)) providers.set(model.providerID, []);
      providers.get(model.providerID)!.push(model);
    }
    providers.forEach((models, title) => groups.push({title, models}));
  }
  return groups.filter(group => group.models.length);
}

export function ModelPicker(props: Props) {
  const [open, setOpen] = useState(false);
  const [last, setLast] = useState<ChatModel>();
  const trigger = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    function shortcut(event: globalThis.KeyboardEvent) {
      if (!props.disabled && !event.repeat && (event.ctrlKey || event.metaKey) && event.key === "'") {
        event.preventDefault();
        setOpen(true);
      }
    }
    window.addEventListener('keydown', shortcut);
    return () => window.removeEventListener('keydown', shortcut);
  }, [props.disabled]);
  const label = props.value && last && identity(last) === identity(props.value) ? last.name : props.value?.id ?? '默认模型';
  return <>
    <button type="button" className="model-trigger" aria-label="选择模型" aria-haspopup="dialog" aria-expanded={open}
      title={props.value ? modelPath(props.value) : '使用 OpenCode 默认模型'} ref={trigger}
      disabled={props.disabled} onClick={() => setOpen(true)}>
      <span>{label}{props.value?.variant && ' · ' + props.value.variant}</span><span aria-hidden="true">⌄</span>
    </button>
    {open && createPortal(<ModelDialog {...props} close={applied => {
      setOpen(false);
      if (applied && props.onApplied) props.onApplied();
      else trigger.current?.focus();
    }} remember={setLast}/>, document.body)}
  </>;
}

function ModelDialog({value, disabled, allowDefault = true, loadModels, onSelect, close, remember}: Props & {
  close: (applied: boolean) => void; remember: (model?: ChatModel) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const search = useRef<HTMLInputElement>(null);
  const list = useRef<HTMLDivElement>(null);
  const variantSelect = useRef<HTMLSelectElement>(null);
  const [draftVariant, setDraftVariant] = useState<{model: string; variant: string}>();
  const [navigating, setNavigating] = useState(false);
  const loader = useRef(loadModels);
  loader.current = loadModels;
  const generation = useRef(0);
  const base = useId();
  const [catalog, setCatalog] = useState<ChatModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [favorites, setFavorites] = useState(() => readHistory(favoriteKey));
  const [recent] = useState(() => readHistory(recentKey));
  const [highlight, setHighlight] = useState<string>();
  const groups = useMemo(() => buildGroups(catalog, query, favorites, recent, allowDefault),
    [catalog, query, favorites, recent, allowDefault]);
  const rows = useMemo(() => groups.flatMap((group, g) => group.models.map(model => ({ model, key: g + ':' + identity(model) }))), [groups]);
  let active = rows.findIndex(row => row.key === highlight);
  if (active < 0) active = Math.max(0, rows.findIndex(row => identity(row.model) === identity(value)));
  const focusedModel = rows[active]?.model;
  const isFavorite = favorites.includes(identity(focusedModel));
  const variants = focusedModel?.variants ?? [];
  function selectedVariant(model: ChatModel) {
    const key = identity(model);
    const variant = draftVariant?.model === key ? draftVariant.variant : identity(value) === key ? value?.variant : undefined;
    return variant && model.variants?.includes(variant) ? variant : '';
  }
  const variant = focusedModel ? selectedVariant(focusedModel) : '';
  async function refresh() {
    const token = ++generation.current;
    setError(''); setLoading(true);
    try {
      const models = await loader.current();
      if (generation.current === token) setCatalog(models);
    } catch (e) {
      if (generation.current === token) setError(e instanceof Error ? e.message : String(e));
    } finally {
      if (generation.current === token) setLoading(false);
    }
  }
  useLayoutEffect(() => {
    const el = dialog.current!;
    el.showModal();
    search.current?.focus();
    return () => el.close();
  }, []);
  useEffect(() => {
    void refresh();
    return () => { generation.current++; };
  }, []);
  useEffect(() => {
    list.current?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' });
  }, [active, catalog, query]);

  function dismiss(applied: boolean) {
    dialog.current?.close();
    close(applied);
  }
  async function apply(model?: ChatModel) {
    if (disabled || applying || (model && loading)) return;
    setApplying(true); setError('');
    const token = generation.current;
    try {
      const selection = model ? { ...model, variant: selectedVariant(model) || undefined } : undefined;
      await onSelect(selection);
      if (generation.current !== token) return;
      if (model) {
        const key = identity(model);
        saveHistory(recentKey, [key, ...readHistory(recentKey).filter(id => id !== key)].slice(0, 5));
      }
      remember(selection);
      dismiss(true);
    } catch (e) {
      if (generation.current === token) setError(e instanceof Error ? e.message : String(e));
    } finally {
      if (generation.current === token) setApplying(false);
    }
  }
  function toggleFavorite(model = focusedModel) {
    if (!model) return;
    const key = identity(model);
    const next = favorites.includes(key) ? favorites.filter(id => id !== key) : [key, ...favorites];
    saveHistory(favoriteKey, next);
    setFavorites(next);
    // Keep the model under the keyboard cursor when a new section changes the list.
    const nextRows = buildGroups(catalog, query, next, recent, allowDefault);
    const group = nextRows.findIndex(g => g.models.some(m => identity(m) === key));
    setHighlight(group + ':' + key);
  }
  function keyDown(e: KeyboardEvent<HTMLDialogElement>) {
    if (e.nativeEvent.isComposing || e.nativeEvent.keyCode === 229 || applying) return;
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f') {
      e.preventDefault(); toggleFavorite(); return;
    }
    if (e.target !== search.current || !rows.length) return;
    if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
      e.preventDefault();
      const offset = e.key === 'ArrowUp' ? -1 : 1;
      setNavigating(true);
      setHighlight(rows[(active + offset + rows.length) % rows.length].key);
    } else if (focusedModel && variants.length && (!query || navigating) &&
        (e.key === 'ArrowRight' || e.key === 'ArrowLeft') && !e.shiftKey && !e.metaKey && !e.ctrlKey && !e.altKey) {
      e.preventDefault();
      const choices = ['', ...variants];
      const offset = e.key === 'ArrowLeft' ? -1 : 1;
      setDraftVariant({model: identity(focusedModel), variant: choices[(choices.indexOf(variant) + offset + choices.length) % choices.length]});
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (!e.repeat) void apply(focusedModel);
    }
  }
  let index = 0;
  return <dialog ref={dialog} className="model-dialog" aria-labelledby={base + '-title'}
    onKeyDown={keyDown} onCancel={e => { e.preventDefault(); if (!applying) dismiss(false); }}
    onClick={e => { if (e.target === dialog.current && !applying) dismiss(false); }}>
    <div className="model-sheet">
      <header><h2 id={base + '-title'}>选择模型</h2>
        <button type="button" disabled={applying} aria-label="关闭模型选择" onClick={() => dismiss(false)}>Esc</button>
      </header>
      <div className="model-search"><span aria-hidden="true">⌕</span>
        <input ref={search} role="combobox" aria-label="搜索模型" placeholder="搜索模型名称、ID 或提供商…"
          aria-expanded="true" aria-controls={base + '-list'} aria-autocomplete="list"
          aria-activedescendant={rows.length ? base + '-' + active : undefined}
          value={query} readOnly={applying} onChange={e => { setQuery(e.target.value); setHighlight(undefined); setNavigating(false); }}/>
        <button disabled={loading || applying} type="button" aria-label="刷新模型列表" onClick={() => void refresh()}>刷新</button>
      </div>
      {error && <p role="alert" className="model-error">{error}</p>}
      {loading && <p role="status" className="model-message">正在读取可用模型…</p>}
      {!loading && !error && !catalog.length && <p className="model-message">当前工作目录没有已启用的模型。</p>}
      {!loading && !!catalog.length && !rows.length && <p role="status" className="model-message">没有匹配的模型，请尝试其他名称或提供商。</p>}
      <div ref={list} className="model-list" role="listbox" aria-label="模型列表" id={base + '-list'}>
        {groups.map((group, groupIndex) => <div role="group" key={groupIndex} aria-label={group.title || '默认'}>
          {group.title && <h3>{group.title}</h3>}
          {group.models.map(model => {
            const i = index++, row = rows[i], selected = identity(model) === identity(value);
            return <div className="model-choice" key={row.key}>
              <div role="option" id={base + '-' + i} aria-selected={selected}
              aria-disabled={disabled || applying || (!!model && loading)} data-active={active === i}
              className="model-row" onMouseDown={e => e.preventDefault()}
              onClick={() => {
                if (disabled || applying || (model && loading)) return;
                setHighlight(row.key);
                if (!model?.variants?.length) { void apply(model); return; }
                requestAnimationFrame(() => variantSelect.current?.focus());
              }}>
              <div><strong>{model?.name ?? '使用 OpenCode 默认模型'}</strong>
                {model && <small>{modelPath(model)}{model.context ? ' · ' + model.context.toLocaleString('en-US') + ' tokens' : ''}</small>}</div>
              {!!model?.variants?.length && <span className="model-variant-count">{model.variants.length} 种变体</span>}

              {selected && <b aria-label="当前模型">✓</b>}
            </div>
            {model && <button type="button" className="model-star" tabIndex={-1} disabled={applying}
              aria-label={'收藏 ' + model.name} aria-pressed={favorites.includes(identity(model))}
              onMouseDown={e => e.preventDefault()} onClick={() => toggleFavorite(model)}>
              {favorites.includes(identity(model)) ? '★' : '☆'}
            </button>}</div>;
          })}
        </div>)}
      </div>
      {focusedModel && variants.length > 0 && <div className="model-variant-panel">
        <div><strong>{focusedModel.name}</strong>
          <label>模型变体<select ref={variantSelect} disabled={disabled || applying || loading} value={variant}
            onChange={e => setDraftVariant({model: identity(focusedModel), variant: e.target.value})}>
            <option value="">默认变体</option>
            {variants.map(name => <option key={name} value={name}>{name}</option>)}
          </select></label></div>
        <button type="button" disabled={disabled || applying || loading} onClick={() => void apply(focusedModel)}>使用此模型</button>
      </div>}
      <footer><span>{applying ? '正在切换模型…' : variants.length ? '↑ ↓ 模型　← → 变体　Enter 选择' : '↑ ↓ 移动　Enter 选择'}</span>
        <button disabled={!focusedModel || applying} type="button" aria-pressed={isFavorite} title="Ctrl / ⌘ + F"
          onClick={() => toggleFavorite()}>{isFavorite ? '★ 取消收藏' : '☆ 收藏模型'}</button>
      </footer>
    </div>
  </dialog>;
}
