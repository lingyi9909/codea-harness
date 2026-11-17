import {test} from 'node:test';
import assert from 'node:assert/strict';
import {UserContextResolver} from '../../apps/server/src/user-context.ts';
test('T4: caller binding is mandatory and ordinary tool output only exposes a masked phone',async()=>{
  const resolver=new UserContextResolver(async id=>({id,name:'Current user',phone:'+8613800001234',status:'ACTIVE'}));
  const binding={ownerId:'A',id:'native-A',serviceIdentity:'isolated-A',scenario:'log-inspection',directory:'/isolated/A'};
  const scope=await resolver.bind('A',binding,'mock-logs');
  assert.deepEqual(resolver.public(scope),{ownerId:'A',sessionId:'native-A',target:'mock-logs',username:'Current user',maskedPhone:'****1234'});
  assert.equal(resolver.internal(scope).phone,'+8613800001234');
  assert.throws(()=>resolver.public({...scope}),/UNTRUSTED_TOOL_CONTEXT/);
  await assert.rejects(resolver.bind('B',binding,'mock-logs'),/FORBIDDEN/);
  await assert.rejects(resolver.bind('A',binding,'real-logs'),/FORBIDDEN/);
});
