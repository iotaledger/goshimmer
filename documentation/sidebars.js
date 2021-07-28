/**
 * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */

module.exports = {
  docs: [{
    type: 'doc',
    label: 'Welcome',
    id: 'welcome',
  },
  {
    type: 'doc',
    label: 'FAQ',
    id: 'faq',
  },
  {
    type: 'category',
    label: 'Tutorials',
    items: [{
      type: 'doc',
      label: 'Set Up a Node',
      id: 'tutorials/setup',
    },
    {
      type: 'doc',
      label: 'Obtain Tokens',
      id: 'tutorials/obtain_tokens',
    },
    {
      type: 'doc',
      label: 'Send a Transaction',
      id: 'tutorials/send_transaction',
    },
    {
      type: 'doc',
      label: 'Wallet Library',
      id: 'tutorials/wallet_library',
    },

    {
      type: 'doc',
      label: 'Write a dApp',
      id: 'tutorials/dApp',
    },

    {
      type: 'doc',
      label: 'Manual Peering',
      id: 'tutorials/manual_peering',
    },

    {
      type: 'doc',
      label: 'Create a Static Identity',
      id: 'tutorials/static_identity',
    },

    {
      type: 'doc',
      label: 'Set Up a Custom dRNG Committee',
      id: 'tutorials/custom_dRNG',
    },

    {
      type: 'doc',
      label: 'Set Up the Monitoring Dashboard',
      id: 'tutorials/monitoring',
    },

    {
      type: 'doc',
      label: 'How To Create and Send Transactions',
      id: 'tutorials/send_transaction',
    },
    ],
  },


  {
    type: 'category',
    label: 'Implementation design',
    items: [
      {
        type: 'doc',
        label: 'Event Driven Model',
        id: 'implementation_design/event_driven_model',
      },
      {
        type: 'doc',
        label: 'Packages and Plugins',
        id: 'implementation_design/packages_plugins',
      },

      {
        type: 'doc',
        label: 'Plugin System',
        id: 'implementation_design/plugin',
      },

      {
        type: 'doc',
        label: 'Configuration Parameters',
        id: 'implementation_design/configuration_parameters',
      },

      {
        type: 'doc',
        label: 'Object Storage',
        id: 'implementation_design/object_storage',
      },
    ],
  },
  {
    type: 'category',
    label: 'Protocol Specification',
    items: [
      {
        type: 'doc',
        label: 'Protocol Specification',
        id: 'protocol_specification',
      },
      {
        type: 'doc',
        label: 'High Level Overview',
        id: 'protocol_specification/protocol_high_level_overview',
      },

      {
        type: 'category',
        label: 'Components',
        items: [

          {
            type: 'doc',
            label: 'Overview',
            id: 'protocol_specification/components/overview',
          },

          {
            type: 'doc',
            label: 'Tangle',
            id: 'protocol_specification/components/tangle',
          },

          {
            type: 'doc',
            label: 'Autopeering',
            id: 'protocol_specification/components/autopeering',
          },

          {
            type: 'doc',
            label: 'Mana',
            id: 'protocol_specification/components/mana',
          },

          {
            type: 'doc',
            label: 'Congestion Control',
            id: 'protocol_specification/components/congestion_control',
          },

          {
            type: 'doc',
            label: 'Consensus Mechanism',
            id: 'protocol_specification/components/consensus_mechanism',
          },

          {
            type: 'doc',
            label: 'UTXO and Ledgerstate',
            id: 'protocol_specification/components/UTXO_and_ledgerstate',
          },

          {
            type: 'doc',
            label: 'Advanced Outputs (Experimental)',
            id: 'protocol_specification/components/advanced_outputs',
          },

          {
            type: 'doc',
            label: 'Markers',
            id: 'protocol_specification/components/markers',
          },
        ]
      },
      {
        type: 'doc',
        label: 'Glossary',
        id: 'protocol_specification/glossary',
      },
    ]
  },
  {
    type: 'category',
    label: 'API',
    items: [

      {
        type: 'doc',
        label: 'Client Lib',
        id: 'apis/client_lib',
      },

      {
        type: 'doc',
        label: 'WebAPI',
        id: 'apis/webAPI',
      },

      {
        type: 'doc',
        label: 'Node Info',
        id: 'apis/info',
      },

      {
        type: 'doc',
        label: 'Autopeering',
        id: 'apis/autopeering',
      },

      {
        type: 'doc',
        label: 'Manual Peering',
        id: 'apis/manual_peering',
      },

      {
        type: 'doc',
        label: 'Communication Layer',
        id: 'apis/communication',
      },

      {
        type: 'doc',
        label: 'Ledgerstate',
        id: 'apis/ledgerstate',
      },

      {
        type: 'doc',
        label: 'Mana',
        id: 'apis/mana',
      },

      {
        type: 'doc',
        label: 'dRNG',
        id: 'apis/dRNG',
      },

      {
        type: 'doc',
        label: 'Snapshot',
        id: 'apis/snapshot',
      },

      {
        type: 'doc',
        label: 'Faucet',
        id: 'apis/faucet',
      },

      {
        type: 'doc',
        label: 'Spammer',
        id: 'apis/spammer',
      },

      {
        type: 'doc',
        label: 'Tools',
        id: 'apis/tools',
      },
    ],
  },
  {
    type: 'category',
    label: 'Tooling',
    items: [

      {
        type: 'doc',
        label: 'Overview',
        id: 'tooling/overview',
      },

      {
        type: 'doc',
        label: 'Docker Private Network',
        id: 'tooling/docker_private_network',
      },

      {
        type: 'doc',
        label: 'Integration Tests',
        id: 'tooling/integration_tests',
      },
    ],
  },
  {
    type: 'category',
    label: 'Team Resources',
    items: [

      {
        type: 'doc',
        label: 'How To Do a Release',
        id: 'teamresources/release',
      },

      {
        type: 'doc',
        label: 'Code Guidelines',
        id: 'teamresources/guidelines',
      },

      {
        type: 'doc',
        label: 'Local Development',
        id: 'teamresources/local_development',
      },

      {
        type: 'doc',
        label: 'Modify the Analysis Dashboard',
        id: 'teamresources/analysis_dashboard',
      },
    ],
  },
  ],
};