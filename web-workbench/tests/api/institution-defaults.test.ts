import {test} from 'node:test';
import assert from 'node:assert/strict';
import {institutionDefaultModel} from '../../apps/web/src/institution-model.ts';
test('institution defaults: prefer available Kimi by id/name and preserve provider/variant; never invent a model',()=>{
  const other={id:'other',providerID:'company',name:'Other'};
  const kimi={id:'kimi-k2',providerID:'private',name:'KIMI K2',variant:'thinking'};
  assert.equal(institutionDefaultModel([other,kimi]),kimi);
  const named={id:'internal-42',providerID:'bank',name:'Kimi'};
  assert.equal(institutionDefaultModel([kimi,named]),named);
  assert.equal(institutionDefaultModel([other]),other);
  assert.equal(institutionDefaultModel([other,{...other,id:'another'}]),undefined);
  assert.equal(institutionDefaultModel([]),undefined);
});
