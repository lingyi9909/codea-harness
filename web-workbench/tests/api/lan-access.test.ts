import {test} from 'node:test';
import assert from 'node:assert/strict';
import {isPrivateLanIPv4,resolveListenHost} from '../../apps/server/src/lan-access.ts';
import {trustedOrigin} from '../../apps/server/src/embed-auth.ts';

test('LAN HTTP remains disabled unless explicit bind, opt-in and private origin are set',()=>{
  assert.equal(resolveListenHost(4176,{}),'127.0.0.1');
  const enabled={HARNESS_ENABLE_LAN:'1',HARNESS_HTTP_BIND:'0.0.0.0',HARNESS_PUBLIC_ORIGIN:'http://10.20.30.40:4176'};
  assert.equal(resolveListenHost(4176,enabled),'0.0.0.0');
  assert.equal(resolveListenHost(4176,{...enabled,HARNESS_HTTP_BIND:'192.168.1.8'}),'192.168.1.8');
  for(const [key,value] of [
    ['HARNESS_ENABLE_LAN','0'],['HARNESS_PUBLIC_ORIGIN','http://8.8.8.8:4176'],
    ['HARNESS_PUBLIC_ORIGIN','http://10.20.30.40:4177'],
    ['HARNESS_PUBLIC_ORIGIN','http://0.0.0.0:4176'],
    ['HARNESS_PUBLIC_ORIGIN','http://10.20.30.40:4176/path'],
    ['HARNESS_HTTP_BIND','8.8.8.8'],['HARNESS_HTTP_BIND','::'],
  ] as const)assert.throws(()=>resolveListenHost(4176,{...enabled,[key]:value}));
  for(const host of ['10.3.4.5','172.16.0.1','172.31.255.254','192.168.2.3'])
    assert.equal(isPrivateLanIPv4(host),true,host);
  for(const host of ['172.15.0.1','172.32.0.1','192.169.2.3','127.0.0.1','0.0.0.0','10.1.1.999','10.01.1.2'])
    assert.equal(isPrivateLanIPv4(host),false,host);
});

test('embed origin accepts HTTP private LAN only with opt-in',()=>{
  const previous=process.env.HARNESS_ENABLE_LAN;
  try{
    delete process.env.HARNESS_ENABLE_LAN;
    assert.throws(()=>trustedOrigin('http://192.168.2.3:4176'));
    trustedOrigin('http://127.0.0.1:4176');
    trustedOrigin('https://workbench.example');
    process.env.HARNESS_ENABLE_LAN='1';
    trustedOrigin('http://192.168.2.3:4176');
    trustedOrigin('http://172.16.1.12:4176');
    assert.throws(()=>trustedOrigin('http://8.8.8.8:4176'));
    assert.throws(()=>trustedOrigin('http://192.168.2.3:4176/path'));
  }finally{
    if(previous===undefined)delete process.env.HARNESS_ENABLE_LAN;
    else process.env.HARNESS_ENABLE_LAN=previous;
  }
});
