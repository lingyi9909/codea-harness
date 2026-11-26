/**
 * Browser UUID for private LAN HTTP pages.
 *
 * crypto.randomUUID() requires a secure context, so it is undefined on many
 * colleagues' http://192.168.x.x pages. getRandomValues() remains available
 * and provides the same cryptographically random UUID v4 bytes.
 * Never use Math.random() for session/operation IDs.
 */
export function browserRandomUUID():string{
  if(typeof crypto.randomUUID==='function')return crypto.randomUUID();
  const bytes=crypto.getRandomValues(new Uint8Array(16));
  bytes[6]=(bytes[6]&0x0f)|0x40;
  bytes[8]=(bytes[8]&0x3f)|0x80;
  const hex=Array.from(bytes,b=>b.toString(16).padStart(2,'0')).join('');
  return [hex.slice(0,8),hex.slice(8,12),hex.slice(12,16),hex.slice(16,20),hex.slice(20)].join('-');
}
