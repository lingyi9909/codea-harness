import {test} from 'node:test';
import assert from 'node:assert/strict';
import {privateLanInterfaces,launchEnvironment,menuChoice,chooseEnvironment,parsePort} from '../../scripts/launch-workbench.mjs';

const interfaces={
  'Wi-Fi':[{family:'IPv4',internal:false,address:'192.168.1.66'}],
  'Ethernet':[{family:'IPv4',internal:false,address:'10.12.1.7'}],
  'vEthernet (WSL)':[{family:'IPv4',internal:false,address:'172.20.0.1'}],
  'Public':[{family:'IPv4',internal:false,address:'8.8.8.8'}],
  'Loopback':[{family:'IPv4',internal:true,address:'127.0.0.1'}],
};

test('only real private IPv4 adapters are selectable, virtual adapters are last',()=>{
  assert.deepEqual(privateLanInterfaces(interfaces),[
    {address:'10.12.1.7',adapter:'Ethernet'},
    {address:'192.168.1.66',adapter:'Wi-Fi'},
    {address:'172.20.0.1',adapter:'vEthernet (WSL)'},
  ]);
});

test('startup selection accepts numeric menu choices with a safe default',()=>{
  assert.equal(menuChoice('',2),1);
  assert.equal(menuChoice(' 2 ',2),2);
  for(const bad of ['0','3','-1','abc','1; whoami'])assert.equal(menuChoice(bad,2),null);
});

test('local-only choice clears old LAN and Shell environment values',()=>{
  const env=launchEnvironment({
    HARNESS_ENABLE_LAN:'1',HARNESS_HTTP_BIND:'0.0.0.0',
    HARNESS_PUBLIC_ORIGIN:'http://192.168.1.66:4176',
    HARNESS_EMBED_PARENTS:'http://192.168.1.66:4176',
    HARNESS_INSTITUTION_ALLOW_SHELL:'1',HARNESS_HTTP_PORT:'4176',
  },{scope:'local',allowShell:false});
  assert.equal(env.HARNESS_ENABLE_LAN,'0');
  assert.equal(env.HARNESS_HTTP_BIND,'127.0.0.1');
  assert.equal(env.HARNESS_PUBLIC_ORIGIN,'http://127.0.0.1:4176');
  assert.equal(env.HARNESS_EMBED_PARENTS,'http://127.0.0.1:4176');
  assert.equal(env.HARNESS_INSTITUTION_ALLOW_SHELL,'0');
});

test('LAN + unrestricted Shell needs an extra explicit confirmation',async()=>{
  const env={HARNESS_HTTP_PORT:'5188'};
  const run=questions=>chooseEnvironment(env,{interfaces,questions});
  const allowed=await run(['2','2','1','2','2']);
  assert.equal(allowed.HARNESS_HTTP_BIND,'0.0.0.0');
  assert.equal(allowed.HARNESS_ENABLE_LAN,'1');
  assert.equal(allowed.HARNESS_PUBLIC_ORIGIN,'http://192.168.1.66:5188');
  assert.equal(allowed.HARNESS_EMBED_PARENTS,'http://192.168.1.66:5188');
  assert.equal(allowed.HARNESS_INSTITUTION_ALLOW_SHELL,'1');
  const restricted=await run(['2','1','1','2','1']);
  assert.equal(restricted.HARNESS_PUBLIC_ORIGIN,'http://10.12.1.7:5188');
  assert.equal(restricted.HARNESS_INSTITUTION_ALLOW_SHELL,'0');
  const localElevated=await run(['1','1','2']);
  assert.equal(localElevated.HARNESS_HTTP_BIND,'127.0.0.1');
  assert.equal(localElevated.HARNESS_INSTITUTION_ALLOW_SHELL,'1');
});


test('startup port accepts safe range and presets without breaking LAN origin mapping',async()=>{
  for(const input of ['0','22','80','1023','65536','invalid','1;whoami','4176.0','-4176'])
    assert.equal(parsePort(input),null,input);
  assert.equal(parsePort('4176'),4176);
  assert.equal(parsePort('65535'),65535);
  const original={HARNESS_HTTP_PORT:'5188'};
  const run=questions=>chooseEnvironment(original,{interfaces,questions});
  const defaults=await run(['1','1','1']);
  assert.equal(defaults.HARNESS_HTTP_PORT,'5188');
  assert.equal(defaults.HARNESS_PUBLIC_ORIGIN,'http://127.0.0.1:5188');
  const common=await run(['2','1','2','1']);
  assert.equal(common.HARNESS_HTTP_PORT,'4177');
  assert.equal(common.HARNESS_PUBLIC_ORIGIN,'http://10.12.1.7:4177');
  assert.equal(common.HARNESS_EMBED_PARENTS,'http://10.12.1.7:4177');
  const other=await run(['1','3','1']);
  assert.equal(other.HARNESS_HTTP_PORT,'8080');
  const custom=await run(['2','2','4','80','bad','5601','1']);
  assert.equal(custom.HARNESS_HTTP_PORT,'5601');
  assert.equal(custom.HARNESS_HTTP_BIND,'0.0.0.0');
  assert.equal(custom.HARNESS_PUBLIC_ORIGIN,'http://192.168.1.66:5601');
  assert.equal(custom.HARNESS_EMBED_PARENTS,'http://192.168.1.66:5601');
  const advanced=await run(['3','1']);
  assert.equal(advanced.HARNESS_HTTP_PORT,'5188');
  assert.equal(advanced.HARNESS_INSTITUTION_ALLOW_SHELL,'0');
  const direct=launchEnvironment({}, {scope:'local',port:6000});
  assert.equal(direct.HARNESS_HTTP_PORT,'6000');
  assert.equal(direct.HARNESS_PUBLIC_ORIGIN,'http://127.0.0.1:6000');
  assert.throws(()=>launchEnvironment({}, {scope:'local',port:65536}),/无效端口/);
});

test('noninteractive ZIP smoke launch retains preexisting configuration and never prompts',async()=>{
  const original={HARNESS_HTTP_PORT:'4788',HARNESS_PUBLIC_ORIGIN:'http://127.0.0.1:4788',HARNESS_INSTITUTION_ALLOW_SHELL:'1'};
  assert.deepEqual(await chooseEnvironment(original,{input:{isTTY:false},interfaces,questions:undefined}),original);
});

test('LAN selection fails closed without a private IP',async()=>{
  await assert.rejects(chooseEnvironment({},{interfaces:{},questions:['2']}),/找不到可用的私有 IPv4/);
  assert.throws(()=>launchEnvironment({},{scope:'lan',address:'8.8.8.8'}),/私有 IPv4/);
});
