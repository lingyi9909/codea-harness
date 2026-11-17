import type { Pool,RowDataPacket } from 'mysql2/promise';
export type ChatBinding={ownerId:string;serviceIdentity:string;id:string;scenario:string;directory:string};
export interface ChatBindingStore {
  create(binding:ChatBinding):Promise<void>;
  get(serviceIdentity:string,id:string,ownerId:string):Promise<ChatBinding|undefined>;
  remove(serviceIdentity:string,id:string,ownerId:string):Promise<void>;
}
export class MysqlChatBindings implements ChatBindingStore {
  private readonly pool:Pool;
  constructor(pool:Pool){this.pool=pool;}
  async create(b:ChatBinding){await this.pool.execute(`INSERT INTO wh_chat_session_bindings
    (service_identity,native_session_id,owner_id,scenario,directory,created_at) VALUES (?,?,?,?,?,UTC_TIMESTAMP(3))`,
    [b.serviceIdentity,b.id,b.ownerId,b.scenario,b.directory]);}
  async get(serviceIdentity:string,id:string,ownerId:string){
    const [rows]=await this.pool.execute<RowDataPacket[]>(`SELECT scenario,directory FROM wh_chat_session_bindings
      WHERE service_identity=? AND native_session_id=? AND owner_id=? AND deleted_at IS NULL`,[serviceIdentity,id,ownerId]);
    return rows[0]?{serviceIdentity,id,ownerId,scenario:String(rows[0].scenario),directory:String(rows[0].directory)}:undefined;
  }
  async remove(serviceIdentity:string,id:string,ownerId:string){
    await this.pool.execute(`UPDATE wh_chat_session_bindings SET deleted_at=UTC_TIMESTAMP(3)
      WHERE service_identity=? AND native_session_id=? AND owner_id=? AND deleted_at IS NULL`,[serviceIdentity,id,ownerId]);
  }
}
