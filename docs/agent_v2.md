Agent này là một phương thức đơn giản hơn agent, chỉ làm 1 việc duy nhất là mở endpoint /GET create_room với room_id:string tron query param. sau đó liên hệ nomad để khởi động 1 server job như bản agent ban đầu.

Nomad job mà agent_v2 khởi tạo chỉ cần input room_id vào args không cần input port