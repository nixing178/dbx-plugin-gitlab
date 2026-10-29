/**
 * 复制文本。插件 iframe 里 navigator.clipboard 可能被策略禁用，
 * 优先走宿主的 dbxPlugin.copy，再退回浏览器 API。
 */
export async function copyText(text: string): Promise<boolean> {
  const hostCopy = window.dbxPlugin?.copy;
  if (hostCopy) {
    try {
      await hostCopy(text);
      return true;
    } catch {
      // 宿主实现失败时继续尝试浏览器 API。
    }
  }

  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}
