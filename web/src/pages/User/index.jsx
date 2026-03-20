/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useState } from 'react';
import { Tabs, TabPane } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import UsersTable from '../../components/table/users';
import UsageLogsTable from '../../components/table/usage-logs';
import { isAdmin } from '../../helpers';

const VIEW_MODE = {
  USERS: 'users',
  USER_RANKING: 'user_ranking',
};

const User = () => {
  const { t } = useTranslation();
  const isAdminUser = isAdmin();
  const [activeTab, setActiveTab] = useState(VIEW_MODE.USERS);

  if (!isAdminUser) {
    return (
      <div className='mt-[60px] px-2'>
        <UsersTable />
      </div>
    );
  }

  return (
    <div className='mt-[60px] px-2'>
      <Tabs
        type='line'
        activeKey={activeTab}
        onChange={setActiveTab}
        className='mb-3'
      >
        <TabPane tab={t('用户列表')} itemKey={VIEW_MODE.USERS} />
        <TabPane tab={t('用户额度排行')} itemKey={VIEW_MODE.USER_RANKING} />
      </Tabs>
      {activeTab === VIEW_MODE.USERS ? (
        <UsersTable />
      ) : (
        <UsageLogsTable
          key={VIEW_MODE.USER_RANKING}
          fixedViewMode={VIEW_MODE.USER_RANKING}
          showViewModeSelector={false}
        />
      )}
    </div>
  );
};

export default User;
